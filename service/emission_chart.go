package service

import (
	"crynux_relay/models"
	"math/big"
	"time"
)

func BuildEmissionIncomeSeries(records []models.VestingRecord, chartRange *EmissionChartRange) ([]int64, []models.BigInt) {
	timestamps := make([]int64, 0, len(chartRange.WeekStarts))
	emissionAmounts := make([]models.BigInt, 0, len(chartRange.WeekStarts))
	bucketSums := make(map[int64]*big.Int, len(chartRange.WeekStarts))

	for _, weekStart := range chartRange.WeekStarts {
		bucketSums[weekStart.Unix()] = big.NewInt(0)
	}

	for _, record := range records {
		alignedStart, ok := AlignToMainnetEmissionWeekStart(record.StartTime, chartRange.MainnetWeekStart)
		if !ok {
			continue
		}
		if alignedStart.Before(chartRange.RangeStart) || !alignedStart.Before(chartRange.RangeEnd) {
			continue
		}
		bucket, exists := bucketSums[alignedStart.Unix()]
		if !exists {
			continue
		}
		bucket.Add(bucket, &record.TotalAmount.Int)
	}

	for _, weekStart := range chartRange.WeekStarts {
		ts := weekStart.Unix()
		timestamps = append(timestamps, ts)
		emissionAmounts = append(emissionAmounts, models.BigInt{Int: *new(big.Int).Set(bucketSums[ts])})
	}

	return timestamps, emissionAmounts
}

func NormalizeToUTCWeek(t time.Time) time.Time {
	return NormalizeToUTCWeekStart(t)
}
