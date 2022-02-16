package controllers

import (
	"github.com/cashapp/cmmc/util/annotations"
)

const (
	watchedBy            annotations.Annotation = "config.cmmc.k8s.cash.app/watched-by-merge-source"
	managedByMergeTarget annotations.Annotation = "config.cmmc.k8s.cash.app/managed-by-merge-target"
)

const (
	mergeSourceFinalizerName = "config.cmmc.k8s.cash.app/merge-source-finalizer"
	mergeTargetFinalizerName = "config.cmmc.k8s.cash.app/merge-target-finalizer"
)
