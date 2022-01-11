package controllers

const (
	watchedByAnnotation            = "config.cmmc.k8s.cash.app/watched-by-merge-source"
	managedByMergeTargetAnnotation = "config.cmmc.k8s.cash.app/managed-by-merge-target"

	mergeSourceFinalizerName = "config.cmmc.k8s.cash.app/merge-source-finalizer"
	mergeTargetFinalizerName = "config.cmmc.k8s.cash.app/merge-target-finalizer"
)
