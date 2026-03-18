package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ModularDevLabs/Fundamentum/internal/models"
	"github.com/bwmarrin/discordgo"
)

var (
	web3EVMContractRe = regexp.MustCompile(`(?i)\b0x[a-f0-9]{40}\b`)
	web3SolAddressRe  = regexp.MustCompile(`\b[1-9A-HJ-NP-Za-km-z]{32,44}\b`)
	web3CashTagRe     = regexp.MustCompile(`(?i)(?:^|\s)\$([a-z][a-z0-9._-]{1,31})\b`)
	web3HTTPClient    = &http.Client{Timeout: 4 * time.Second}
)

type web3Signal struct {
	Contract string
	CashTag  string
}

type dexScreenerTokenResponse struct {
	Pairs []dexPair `json:"pairs"`
}

type dexPair struct {
	ChainID   string  `json:"chainId"`
	DexID     string  `json:"dexId"`
	URL       string  `json:"url"`
	PairAddr  string  `json:"pairAddress"`
	PriceUSD  string  `json:"priceUsd"`
	FDV       float64 `json:"fdv"`
	MarketCap float64 `json:"marketCap"`
	BaseToken struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
	} `json:"baseToken"`
	Liquidity struct {
		USD float64 `json:"usd"`
	} `json:"liquidity"`
	Volume struct {
		H24 float64 `json:"h24"`
	} `json:"volume"`
	PriceChange struct {
		H24 float64 `json:"h24"`
	} `json:"priceChange"`
}

type cgSearchResponse struct {
	Coins []cgSearchCoin `json:"coins"`
}

type cgSearchCoin struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
	MarketCapRank int    `json:"market_cap_rank"`
}

type cgMarket struct {
	ID                       string   `json:"id"`
	Symbol                   string   `json:"symbol"`
	Name                     string   `json:"name"`
	CurrentPrice             float64  `json:"current_price"`
	MarketCap                float64  `json:"market_cap"`
	FDV                      *float64 `json:"fully_diluted_valuation"`
	PriceChangePercentage24H *float64 `json:"price_change_percentage_24h"`
}

func (s *Service) handleWeb3IntelMessage(ctx context.Context, m *discordgo.MessageCreate, settings models.GuildSettings) {
	if !settings.FeatureEnabled(models.FeatureWeb3Intel) {
		return
	}
	signal := detectWeb3Signal(m.Content)
	if signal.Contract == "" && signal.CashTag == "" {
		return
	}
	if !s.allowWeb3Lookup(m.ChannelID) {
		return
	}

	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var out string
	var err error
	switch {
	case signal.Contract != "":
		out, err = s.resolveContractIntel(lookupCtx, signal.Contract)
	case signal.CashTag != "":
		out, err = s.resolveCashTagIntel(lookupCtx, signal.CashTag)
	}
	if err != nil {
		s.logger.Debug("web3 intel lookup failed guild=%s channel=%s err=%v", m.GuildID, m.ChannelID, err)
		return
	}
	if strings.TrimSpace(out) == "" {
		return
	}
	if _, err := s.session.ChannelMessageSend(m.ChannelID, out); err != nil {
		s.logger.Error("web3 intel reply failed guild=%s channel=%s err=%v", m.GuildID, m.ChannelID, err)
	}
}

func (s *Service) allowWeb3Lookup(channelID string) bool {
	if channelID == "" {
		return false
	}
	s.web3Mu.Lock()
	defer s.web3Mu.Unlock()
	last := s.web3Last[channelID]
	now := time.Now().UTC()
	if !last.IsZero() && now.Sub(last) < 8*time.Second {
		return false
	}
	s.web3Last[channelID] = now
	return true
}

func detectWeb3Signal(content string) web3Signal {
	text := strings.TrimSpace(content)
	if text == "" {
		return web3Signal{}
	}
	evmLoc := web3EVMContractRe.FindStringIndex(text)
	solLoc := web3SolAddressRe.FindStringIndex(text)
	if evmLoc != nil && (solLoc == nil || evmLoc[0] <= solLoc[0]) {
		return web3Signal{Contract: strings.ToLower(text[evmLoc[0]:evmLoc[1]])}
	}
	if solLoc != nil {
		return web3Signal{Contract: text[solLoc[0]:solLoc[1]]}
	}
	match := web3CashTagRe.FindStringSubmatch(text)
	if len(match) >= 2 {
		return web3Signal{CashTag: strings.ToLower(match[1])}
	}
	return web3Signal{}
}

func (s *Service) resolveContractIntel(ctx context.Context, contract string) (string, error) {
	var dex dexScreenerTokenResponse
	if err := web3FetchJSON(ctx, "https://api.dexscreener.com/latest/dex/tokens/"+url.PathEscape(contract), &dex); err != nil {
		return "", err
	}
	if len(dex.Pairs) == 0 {
		return "", nil
	}
	best := chooseBestDexPair(dex.Pairs)
	if best == nil {
		return "", nil
	}
	price := parseDecimal(best.PriceUSD)
	mcap := best.MarketCap
	if mcap <= 0 {
		mcap = best.FDV
	}
	line := fmt.Sprintf(
		"**Web3 Intel** `%s`\n%s (%s) on %s via %s\nPrice: %s | 24h: %s | MCap: %s | Liquidity: %s\nDexscreener: %s",
		contract,
		fallback(best.BaseToken.Name, "Token"),
		strings.ToUpper(fallback(best.BaseToken.Symbol, "?")),
		formatChain(best.ChainID),
		fallback(best.DexID, "dex"),
		formatUSD(price),
		formatPercent(best.PriceChange.H24),
		formatUSDCompact(mcap),
		formatUSDCompact(best.Liquidity.USD),
		fallback(best.URL, "n/a"),
	)
	return trimMessage(line), nil
}

func (s *Service) resolveCashTagIntel(ctx context.Context, token string) (string, error) {
	var search cgSearchResponse
	if err := web3FetchJSON(ctx, "https://api.coingecko.com/api/v3/search?query="+url.QueryEscape(token), &search); err != nil {
		return "", err
	}
	coin := chooseCoinGeckoCoin(search.Coins, token)
	if coin == nil {
		return "", nil
	}
	var markets []cgMarket
	u := "https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=" + url.QueryEscape(coin.ID)
	if err := web3FetchJSON(ctx, u, &markets); err != nil {
		return "", err
	}
	if len(markets) == 0 {
		return "", nil
	}
	m := markets[0]
	fdv := 0.0
	if m.FDV != nil {
		fdv = *m.FDV
	}
	change := 0.0
	if m.PriceChangePercentage24H != nil {
		change = *m.PriceChangePercentage24H
	}
	line := fmt.Sprintf(
		"**Web3 Intel** `$%s`\n%s (%s)\nPrice: %s | 24h: %s | MCap: %s | FDV: %s\nCoinGecko: https://www.coingecko.com/en/coins/%s",
		strings.ToUpper(token),
		fallback(m.Name, coin.Name),
		strings.ToUpper(fallback(m.Symbol, coin.Symbol)),
		formatUSD(m.CurrentPrice),
		formatPercent(change),
		formatUSDCompact(m.MarketCap),
		formatUSDCompact(fdv),
		coin.ID,
	)
	return trimMessage(line), nil
}

func chooseBestDexPair(pairs []dexPair) *dexPair {
	if len(pairs) == 0 {
		return nil
	}
	candidates := make([]dexPair, 0, len(pairs))
	for _, p := range pairs {
		if strings.TrimSpace(p.PriceUSD) == "" {
			continue
		}
		candidates = append(candidates, p)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Liquidity.USD == candidates[j].Liquidity.USD {
			return candidates[i].Volume.H24 > candidates[j].Volume.H24
		}
		return candidates[i].Liquidity.USD > candidates[j].Liquidity.USD
	})
	return &candidates[0]
}

func chooseCoinGeckoCoin(coins []cgSearchCoin, query string) *cgSearchCoin {
	if len(coins) == 0 {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	bestIdx := -1
	bestScore := -1
	bestRank := math.MaxInt32
	for i := range coins {
		c := coins[i]
		score := 0
		if strings.EqualFold(c.Symbol, q) {
			score = 3
		} else if strings.EqualFold(c.Name, q) {
			score = 2
		} else if strings.Contains(strings.ToLower(c.Name), q) || strings.Contains(strings.ToLower(c.Symbol), q) {
			score = 1
		}
		if score == 0 {
			continue
		}
		rank := c.MarketCapRank
		if rank <= 0 {
			rank = math.MaxInt32
		}
		if score > bestScore || (score == bestScore && rank < bestRank) {
			bestIdx = i
			bestScore = score
			bestRank = rank
		}
	}
	if bestIdx >= 0 {
		return &coins[bestIdx]
	}
	return &coins[0]
}

func web3FetchJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "FundamentumBot/1.0 (+https://github.com/ModularDevLabs/Fundamentum)")

	res, err := web3HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("http %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func parseDecimal(raw string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return n
}

func formatUSD(v float64) string {
	if v <= 0 {
		return "n/a"
	}
	if v >= 1 {
		return fmt.Sprintf("$%.4f", v)
	}
	return fmt.Sprintf("$%.8f", v)
}

func formatUSDCompact(v float64) string {
	if v <= 0 {
		return "n/a"
	}
	switch {
	case v >= 1_000_000_000:
		return fmt.Sprintf("$%.2fB", v/1_000_000_000)
	case v >= 1_000_000:
		return fmt.Sprintf("$%.2fM", v/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("$%.2fK", v/1_000)
	default:
		return fmt.Sprintf("$%.0f", v)
	}
}

func formatPercent(v float64) string {
	if math.Abs(v) < 0.0001 {
		return "0.00%"
	}
	return fmt.Sprintf("%+.2f%%", v)
}

func formatChain(chain string) string {
	switch strings.ToLower(strings.TrimSpace(chain)) {
	case "ethereum":
		return "Ethereum"
	case "arbitrum":
		return "Arbitrum"
	case "optimism":
		return "Optimism"
	case "base":
		return "Base"
	case "polygon":
		return "Polygon"
	case "linea":
		return "Linea"
	case "zksync":
		return "zkSync"
	case "bsc":
		return "BNB Chain"
	case "solana":
		return "Solana"
	case "hyperliquid":
		return "Hyperliquid"
	case "monad":
		return "Monad"
	default:
		return fallback(chain, "unknown")
	}
}

func fallback(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func trimMessage(s string) string {
	if len(s) <= 1900 {
		return s
	}
	return s[:1897] + "..."
}
