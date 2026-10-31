package common

import "time"

// DateRange treats date-only 'to' as inclusive; timestamp 'to' is exclusive.
func DateRange(from, to, zone string) (time.Time, time.Time, *time.Location, error) {
	if zone == "" {
		zone = "Asia/Ho_Chi_Minh"
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, time.Time{}, nil, ErrInvalidArgument
	}
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 1)
	if from != "" {
		start, err = time.ParseInLocation("2006-01-02", from, loc)
		if err != nil {
			start, err = time.Parse(time.RFC3339, from)
		}
		if err != nil {
			return start, end, loc, ErrInvalidArgument
		}
	}
	if to != "" {
		end, err = time.ParseInLocation("2006-01-02", to, loc)
		if err == nil {
			end = end.AddDate(0, 0, 1)
		} else {
			end, err = time.Parse(time.RFC3339, to)
		}
		if err != nil {
			return start, end, loc, ErrInvalidArgument
		}
	}
	if !start.Before(end) || end.Sub(start) > 366*24*time.Hour {
		return start, end, loc, ErrInvalidArgument
	}
	return start.UTC(), end.UTC(), loc, nil
}
