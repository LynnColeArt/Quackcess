package vector

import "time"

func IsVectorIndexStale(lastIndexedAt, sourceUpdatedAt time.Time, staleAfterHours int, now time.Time) bool {
	if now.IsZero() {
		now = time.Now()
	}
	if lastIndexedAt.IsZero() {
		return true
	}
	if !sourceUpdatedAt.IsZero() && sourceUpdatedAt.After(lastIndexedAt) {
		return true
	}
	if staleAfterHours <= 0 {
		return false
	}
	return !now.Before(lastIndexedAt.Add(time.Duration(staleAfterHours) * time.Hour))
}

func (v VectorField) IsStale(sourceUpdatedAt time.Time, now time.Time) bool {
	return IsVectorIndexStale(v.LastIndexedAt, sourceUpdatedAt, v.StaleAfterHours, now)
}
