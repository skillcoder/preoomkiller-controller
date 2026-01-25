package controller

import "errors"

var (
	ErrMemoryThresholdParse = errors.New("parse memory threshold")
	ErrGetPodMetrics        = errors.New("get pod metrics")
	ErrEvictPod             = errors.New("evict pod")
)
