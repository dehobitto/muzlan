package stats

import (
	"slices"
	"time"
)

// Periods of a Summary row: today, the last 7 days, all time. Days are UTC.
const (
	Today = iota
	Week
	AllTime
)

// Summary is what /stats shows.
type Summary struct {
	Downloads [3]int // successful
	Failed    [3]int
	Users     [3]int // distinct users with any event
	NewUsers  [3]int // users whose first event is in the period
	Searches  [3]int

	// DailyDownloads holds successful downloads per day, oldest first,
	// ending today.
	DailyDownloads []DayCount
	// Failures counts failed downloads by reason over all time.
	Failures map[string]int

	SearchMedian, SearchP95     time.Duration
	DownloadMedian, DownloadP95 time.Duration
}

type DayCount struct {
	Day   time.Time
	Count int
}

// Summarize computes a Summary at now, with days of daily downloads.
func Summarize(events []Event, now time.Time, days int) Summary {
	now = now.UTC()
	today := now.Truncate(24 * time.Hour)
	periodStarts := [3]time.Time{today, today.AddDate(0, 0, -6), {}}

	summary := Summary{Failures: map[string]int{}}
	firstSeen := map[string]time.Time{}
	seen := [3]map[string]bool{{}, {}, {}}
	daily := make([]int, days)
	var searchTimes, downloadTimes []time.Duration

	for _, event := range events {
		if event.User != "" {
			if first, ok := firstSeen[event.User]; !ok || event.Time.Before(first) {
				firstSeen[event.User] = event.Time
			}
		}
		for period, start := range periodStarts {
			if event.Time.Before(start) {
				continue
			}
			if event.User != "" {
				seen[period][event.User] = true
			}
			switch {
			case event.Type == TypeSearch:
				summary.Searches[period]++
			case event.Type == TypeDownload && event.OK:
				summary.Downloads[period]++
			case event.Type == TypeDownload:
				summary.Failed[period]++
			}
		}

		switch event.Type {
		case TypeSearch:
			searchTimes = append(searchTimes, time.Duration(event.MS)*time.Millisecond)
		case TypeDownload:
			if !event.OK {
				summary.Failures[event.Reason]++
				continue
			}
			downloadTimes = append(downloadTimes, time.Duration(event.MS)*time.Millisecond)
			if age := int(today.Sub(event.Time.UTC().Truncate(24*time.Hour)) / (24 * time.Hour)); age >= 0 && age < days {
				daily[days-1-age]++
			}
		}
	}

	for period, start := range periodStarts {
		summary.Users[period] = len(seen[period])
		for _, first := range firstSeen {
			if !first.Before(start) {
				summary.NewUsers[period]++
			}
		}
	}
	for i, count := range daily {
		summary.DailyDownloads = append(summary.DailyDownloads, DayCount{Day: today.AddDate(0, 0, i-days+1), Count: count})
	}
	summary.SearchMedian, summary.SearchP95 = percentile(searchTimes, 50), percentile(searchTimes, 95)
	summary.DownloadMedian, summary.DownloadP95 = percentile(downloadTimes, 50), percentile(downloadTimes, 95)
	return summary
}

// percentile uses the nearest-rank method; it is 0 for no values.
func percentile(values []time.Duration, p int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	rank := (p*len(sorted) + 99) / 100
	return sorted[max(rank, 1)-1]
}
