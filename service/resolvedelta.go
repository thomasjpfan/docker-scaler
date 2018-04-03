package service

import (
	"strconv"
)

// ResolveDeltaOptions are options for resolving changes in scaling
type ResolveDeltaOptions struct {
	MinLabel           string
	MaxLabel           string
	ScaleDownByLabel   string
	ScaleUpByLabel     string
	DefaultMin         uint64
	DefaultMax         uint64
	DefaultScaleDownBy uint64
	DefaultScaleUpBy   uint64
}

// resolveDelta takes a `current` and `by` and returns current + by
// this makes sure the sum is in within the min and max bounds
// It by is zero, then ResolveDeltaOptions will be used
func resolveDelta(current uint64, by uint64, scaleDirection ScaleDirection,
	labels map[string]string, opts ResolveDeltaOptions) (uint64, uint64, uint64) {

	internalDelta := getInternalDelta(by, scaleDirection, labels, opts)
	minBound, maxBound := getBounds(labels, opts)

	newCurrentInt := int64(current) + internalDelta
	var newCurrent uint64
	if newCurrentInt < 0 {
		newCurrent = 0
	} else {
		newCurrent = uint64(newCurrentInt)
	}

	// Pin current to bounds
	if newCurrent < minBound {
		newCurrent = minBound
	}
	if newCurrent > maxBound {
		newCurrent = maxBound
	}

	return minBound, maxBound, newCurrent
}

func getInternalDelta(by uint64, scaleDirection ScaleDirection,
	labels map[string]string, opts ResolveDeltaOptions) int64 {
	if by != 0 {
		if scaleDirection == ScaleDownDirection {
			return -int64(by)
		}
		return int64(by)
	}

	var delta int64
	if scaleDirection == ScaleDownDirection {
		delta = -int64(opts.DefaultScaleDownBy)
		if byLabel, ok := labels[opts.ScaleDownByLabel]; ok {
			if scaleDownNum, err := strconv.Atoi(byLabel); err == nil {
				delta = -int64(scaleDownNum)
			}
		}

	} else {
		delta = int64(opts.DefaultScaleUpBy)
		if byLabel, ok := labels[opts.ScaleUpByLabel]; ok {
			if scaleUpNum, err := strconv.Atoi(byLabel); err == nil {
				delta = int64(scaleUpNum)
			}
		}
	}

	return delta
}
func getBounds(labels map[string]string, opts ResolveDeltaOptions) (uint64, uint64) {
	min, max := opts.DefaultMin, opts.DefaultMax

	if minLabel, ok := labels[opts.MinLabel]; ok {
		if minNum, err := strconv.Atoi(minLabel); err == nil && minNum >= 0 {
			min = uint64(minNum)
		}
	}
	if maxLabel, ok := labels[opts.MaxLabel]; ok {
		if maxNum, err := strconv.Atoi(maxLabel); err == nil && maxNum >= 0 {
			max = uint64(maxNum)
		}
	}

	return min, max
}
