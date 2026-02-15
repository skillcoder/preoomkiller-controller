package controller

import "errors"

var (
	ErrMemoryThresholdParse  = errors.New("parse memory threshold")
	ErrMemoryLimitNotDefined = errors.New("memory limit not defined")
	ErrGetPodMetrics         = errors.New("get pod metrics")
	ErrEvictPod              = errors.New("evict pod")
)
