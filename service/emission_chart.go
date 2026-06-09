package service

import (
	"crynux_relay/models"
	"math/big"
	"time"
)

type TypedEmissionIncomeSeries struct {
	Timestamps               []int64
	NodeEmissionIncome       []models.BigInt
	DelegationEmissionIncome []models.BigInt
}

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

func BuildTypedEmissionIncomeSeries(records []models.VestingRecord, chartRange *EmissionChartRange) *TypedEmissionIncomeSeries {
	timestamps := make([]int64, 0, len(chartRange.WeekStarts))
	nodeEmissionAmounts := make([]models.BigInt, 0, len(chartRange.WeekStarts))
	delegationEmissionAmounts := make([]models.BigInt, 0, len(chartRange.WeekStarts))
	nodeBucketSums := make(map[int64]*big.Int, len(chartRange.WeekStarts))
	delegationBucketSums := make(map[int64]*big.Int, len(chartRange.WeekStarts))

	for _, weekStart := range chartRange.WeekStarts {
		unixTs := weekStart.Unix()
		nodeBucketSums[unixTs] = big.NewInt(0)
		delegationBucketSums[unixTs] = big.NewInt(0)
	}

	for _, record := range records {
		alignedStart, ok := AlignToMainnetEmissionWeekStart(record.StartTime, chartRange.MainnetWeekStart)
		if !ok {
			continue
		}
		if alignedStart.Before(chartRange.RangeStart) || !alignedStart.Before(chartRange.RangeEnd) {
			continue
		}
		unixTs := alignedStart.Unix()
		switch record.Type {
		case models.VestingTypeNode:
			bucket, exists := nodeBucketSums[unixTs]
			if exists {
				bucket.Add(bucket, &record.TotalAmount.Int)
			}
		case models.VestingTypeDelegation:
			bucket, exists := delegationBucketSums[unixTs]
			if exists {
				bucket.Add(bucket, &record.TotalAmount.Int)
			}
		}
	}

	for _, weekStart := range chartRange.WeekStarts {
		ts := weekStart.Unix()
		timestamps = append(timestamps, ts)
		nodeEmissionAmounts = append(nodeEmissionAmounts, models.BigInt{Int: *new(big.Int).Set(nodeBucketSums[ts])})
		delegationEmissionAmounts = append(delegationEmissionAmounts, models.BigInt{Int: *new(big.Int).Set(delegationBucketSums[ts])})
	}

	return &TypedEmissionIncomeSeries{
		Timestamps:               timestamps,
		NodeEmissionIncome:       nodeEmissionAmounts,
		DelegationEmissionIncome: delegationEmissionAmounts,
	}
}

func NormalizeToUTCWeek(t time.Time) time.Time {
	return NormalizeToUTCWeekStart(t)
}
