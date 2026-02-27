package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type BackfillJob struct {
	ID              string    `json:"id"`
	GuildID         string    `json:"guild_id"`
	StartedAt       time.Time `json:"started_at"`
	Status          string    `json:"status"`
	Error           string    `json:"error"`
	TotalChannels   int       `json:"total_channels"`
	ScannedChannels int       `json:"scanned_channels"`
	CheckedMessages int       `json:"checked_messages"`
	UpdatedUsers    int       `json:"updated_users"`
	SkippedChannels int       `json:"skipped_channels"`
}

func (s *Service) StartBackfill(ctx context.Context, guildID string, days int) (string, error) {
	s.ensureBackfillInit()
	if days < 1 {
		days = 1
	}
	job := BackfillJob{
		ID:        time.Now().UTC().Format("20060102T150405.000Z0700"),
		GuildID:   guildID,
		StartedAt: time.Now().UTC(),
		Status:    "queued",
	}

	s.backfillMu.Lock()
	s.backfillJobs[job.ID] = job
	s.backfillMu.Unlock()

	go s.runBackfill(context.Background(), job, days)
	return job.ID, nil
}

func (s *Service) BackfillStatus() []BackfillJob {
	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()

	out := make([]BackfillJob, 0, len(s.backfillJobs))
	for _, job := range s.backfillJobs {
		out = append(out, job)
	}
	return out
}

func (s *Service) runBackfill(ctx context.Context, job BackfillJob, days int) {
	update := func(status, errMsg string) {
		s.backfillMu.Lock()
		j := s.backfillJobs[job.ID]
		j.Status = status
		j.Error = errMsg
		s.backfillJobs[job.ID] = j
		s.backfillMu.Unlock()
	}

	update("running", "")

	settings, err := s.repos.Settings.Get(ctx, job.GuildID)
	if err != nil {
		update("failed", err.Error())
		return
	}

	if settings.InactiveDays > days {
		days = settings.InactiveDays
	}
	s.logger.Info("backfill start guild=%s job=%s days=%d inactivity_days=%d", job.GuildID, job.ID, days, settings.InactiveDays)

	channels, err := s.session.GuildChannels(job.GuildID)
	if err != nil {
		update("failed", err.Error())
		return
	}

	var threadsByParent map[string][]*discordgo.Channel
	if activeThreads, err := s.session.GuildThreadsActive(job.GuildID); err == nil && activeThreads != nil {
		threadsByParent = map[string][]*discordgo.Channel{}
		for _, th := range activeThreads.Threads {
			if th.ParentID != "" {
				threadsByParent[th.ParentID] = append(threadsByParent[th.ParentID], th)
			}
		}
	}

	include := make(map[string]bool)
	for _, t := range settings.BackfillIncludeTypes {
		include[t] = true
	}

	filtered := make([]*discordgo.Channel, 0, len(channels))
	for _, ch := range channels {
		name := channelTypeName(ch.Type)
		if name == "" {
			continue
		}
		if len(include) > 0 && !include[name] {
			continue
		}
		if len(include) == 0 && ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
			continue
		}
		filtered = append(filtered, ch)

		if ch.Type == discordgo.ChannelTypeGuildForum && threadsByParent != nil {
			for _, th := range threadsByParent[ch.ID] {
				filtered = append(filtered, th)
			}
		}
	}

	if len(filtered) == 0 {
		update("failed", "no channels matched backfill filters")
		return
	}

	concurrency := settings.BackfillConcurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	s.logger.Info("backfill channels guild=%s job=%s matched=%d concurrency=%d", job.GuildID, job.ID, len(filtered), concurrency)

	cutoff := time.Now().AddDate(0, 0, -days)
	activeCutoff := time.Now().AddDate(0, 0, -settings.InactiveDays)
	activeUsers, err := s.repos.Activity.ActiveUsersSince(ctx, job.GuildID, activeCutoff)
	if err != nil {
		update("failed", err.Error())
		return
	}
	activeMu := &sync.Mutex{}
	s.updateBackfill(job.ID, func(j *BackfillJob) {
		j.TotalChannels = len(filtered)
	})

	workCh := make(chan *discordgo.Channel)
	limiter := NewRateLimiter(5)
	var failedMu sync.Mutex
	failedChannels := 0
	failedSamples := make([]string, 0, 5)

	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for ch := range workCh {
			checked, updated, err := s.backfillChannel(ctx, job.GuildID, ch, cutoff, limiter, activeUsers, activeMu)
			s.updateBackfill(job.ID, func(j *BackfillJob) {
				j.ScannedChannels++
				j.CheckedMessages += checked
				j.UpdatedUsers += updated
			})
			if err != nil {
				if isChannelInaccessibleError(err) {
					channelLabel := ch.Name
					if channelLabel == "" {
						channelLabel = "<unnamed>"
					}
					s.logger.Info("backfill channel skipped (missing access) guild=%s job=%s channel=%s (%s)", job.GuildID, job.ID, channelLabel, ch.ID)
					s.updateBackfill(job.ID, func(j *BackfillJob) {
						j.SkippedChannels++
					})
					continue
				}
				channelLabel := ch.Name
				if channelLabel == "" {
					channelLabel = "<unnamed>"
				}
				channelErr := fmt.Sprintf("channel=%s (%s): %v", channelLabel, ch.ID, err)
				s.logger.Error("backfill channel failed guild=%s job=%s %s", job.GuildID, job.ID, channelErr)
				failedMu.Lock()
				failedChannels++
				if len(failedSamples) < 5 {
					failedSamples = append(failedSamples, channelErr)
				}
				failedMu.Unlock()
				continue
			}
		}
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker()
	}

	go func() {
		defer close(workCh)
		for _, ch := range filtered {
			select {
			case <-ctx.Done():
				return
			case workCh <- ch:
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		update("failed", ctx.Err().Error())
	case <-done:
		failedMu.Lock()
		count := failedChannels
		samples := append([]string{}, failedSamples...)
		failedMu.Unlock()
		if count > 0 {
			errMsg := fmt.Sprintf("%d channel(s) failed; sample: %s", count, joinSamples(samples))
			update("partial", errMsg)
			s.logger.Error("backfill completed with channel errors guild=%s job=%s failures=%d", job.GuildID, job.ID, count)
			return
		}
		update("success", "")
		s.logger.Info("backfill success guild=%s job=%s", job.GuildID, job.ID)
	}
}

func (s *Service) initBackfillState() {
	s.backfillMu = sync.Mutex{}
	if s.backfillJobs == nil {
		s.backfillJobs = make(map[string]BackfillJob)
	}
}

func (s *Service) ensureBackfillInit() {
	if s.backfillJobs == nil {
		s.initBackfillState()
	}
}

func (s *Service) updateBackfill(jobID string, fn func(*BackfillJob)) {
	s.backfillMu.Lock()
	defer s.backfillMu.Unlock()
	j := s.backfillJobs[jobID]
	fn(&j)
	s.backfillJobs[jobID] = j
}

func (s *Service) backfillChannel(ctx context.Context, guildID string, ch *discordgo.Channel, cutoff time.Time, limiter *RateLimiter, activeUsers map[string]struct{}, activeMu *sync.Mutex) (int, int, error) {
	beforeID, _, err := s.repos.Backfill.GetState(ctx, guildID, ch.ID)
	if err != nil {
		return 0, 0, err
	}
	checked := 0
	updated := 0

	for {
		if err := limiter.Wait(ctx); err != nil {
			return checked, updated, err
		}

		messages, err := s.session.ChannelMessages(ch.ID, 100, beforeID, "", "")
		if err != nil {
			return checked, updated, err
		}
		if len(messages) == 0 {
			return checked, updated, nil
		}

		var reachedCutoff bool
		for _, msg := range messages {
			checked++
			if msg.Author == nil || msg.Author.Bot {
				continue
			}
			ts := msg.Timestamp
			if ts.Before(cutoff) {
				reachedCutoff = true
				continue
			}
			activeMu.Lock()
			_, ok := activeUsers[msg.Author.ID]
			activeMu.Unlock()
			if ok {
				continue
			}

			username := msg.Author.Username
			globalName := ""
			displayName := username
			if msg.Member != nil && msg.Member.Nick != "" {
				displayName = msg.Member.Nick
			}

			if err := s.repos.Activity.UpsertActivity(ctx, guildID, msg.Author.ID, ch.ID, ts, username, globalName, displayName); err != nil {
				return checked, updated, err
			}
			activeMu.Lock()
			activeUsers[msg.Author.ID] = struct{}{}
			activeMu.Unlock()
			updated++
		}

		oldest := messages[len(messages)-1]
		if err := s.repos.Backfill.UpsertState(ctx, guildID, ch.ID, oldest.ID); err != nil {
			return checked, updated, err
		}

		if reachedCutoff {
			return checked, updated, nil
		}
		beforeID = oldest.ID
	}
}

func channelTypeName(t discordgo.ChannelType) string {
	switch t {
	case discordgo.ChannelTypeGuildText:
		return "GUILD_TEXT"
	case discordgo.ChannelTypeGuildNews:
		return "GUILD_NEWS"
	case discordgo.ChannelTypeGuildForum:
		return "GUILD_FORUM"
	case discordgo.ChannelTypeGuildPublicThread:
		return "GUILD_PUBLIC_THREAD"
	case discordgo.ChannelTypeGuildPrivateThread:
		return "GUILD_PRIVATE_THREAD"
	case discordgo.ChannelTypeGuildNewsThread:
		return "GUILD_NEWS_THREAD"
	default:
		return ""
	}
}

func joinSamples(samples []string) string {
	if len(samples) == 0 {
		return "none"
	}
	out := samples[0]
	for i := 1; i < len(samples); i++ {
		out += " | " + samples[i]
	}
	return out
}

func isChannelInaccessibleError(err error) bool {
	if err == nil {
		return false
	}
	restErr, ok := err.(*discordgo.RESTError)
	if ok && restErr != nil && restErr.Message != nil {
		switch restErr.Message.Code {
		case 50001, 50013:
			return true
		}
	}
	msg := err.Error()
	return strings.Contains(msg, "Missing Access") || strings.Contains(msg, "Missing Permissions")
}
