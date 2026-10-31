package logic

import (
	"context"
	"sea-codex-server/internal/common"
	"sea-codex-server/internal/dossier/models"
	"sea-codex-server/internal/dossier/repositories"
	"sort"
	"time"
)

func counts() map[string]int {
	return map[string]int{models.Processing: 0, models.Missing: 0, models.Invalid: 0, models.Valid: 0, models.Approved: 0, models.Rejected: 0}
}

type dashboardDay struct {
	Date   string         `json:"date"`
	Counts map[string]int `json:"counts"`
}

func (l *DossierLogic) Dashboard(ctx context.Context, from, to, zone, metric, typ string) (any, error) {
	a, err := actor(ctx, "dashboard:read")
	if err != nil {
		return nil, err
	}
	if metric == "" {
		metric = "transitions"
	}
	if metric != "transitions" && metric != "unique" && metric != "current" {
		return nil, common.ErrInvalidArgument
	}
	start, end, loc, err := common.DateRange(from, to, zone)
	if err != nil {
		return nil, err
	}
	r := l.repo.Scope(a.CompanyID)
	totals := counts()
	daily := []dashboardDay{}
	if metric == "current" {
		rows, err := r.CurrentStatuses(ctx, typ)
		if err != nil {
			return nil, repositories.Error(err)
		}
		for _, d := range rows {
			totals[d.Status]++
		}
		return map[string]any{"metric": metric, "timezone": loc.String(), "as_of": time.Now().UTC(), "range_applies": false, "totals": totals, "daily": daily}, nil
	}
	history, err := r.History(ctx, start, end, typ)
	if err != nil {
		return nil, repositories.Error(err)
	}
	buckets := map[string]map[string]int{}
	seen := map[string]bool{}
	dailySeen := map[string]bool{}
	for day := start.In(loc); day.Before(end); day = day.AddDate(0, 0, 1) {
		buckets[day.Format("2006-01-02")] = counts()
	}
	for _, h := range history {
		day := h.CreatedAt.In(loc).Format("2006-01-02")
		if buckets[day] == nil {
			buckets[day] = counts()
		}
		key := h.To + ":" + h.DossierID.String()
		if metric == "transitions" || !seen[key] {
			totals[h.To]++
			seen[key] = true
		}
		if metric == "transitions" || !dailySeen[day+key] {
			buckets[day][h.To]++
			dailySeen[day+key] = true
		}
	}
	days := []string{}
	for day := range buckets {
		days = append(days, day)
	}
	sort.Strings(days)
	for _, day := range days {
		daily = append(daily, dashboardDay{Date: day, Counts: buckets[day]})
	}
	return map[string]any{"metric": metric, "timezone": loc.String(), "from": start, "to_exclusive": end, "range_applies": true, "totals": totals, "daily": daily}, nil
}
