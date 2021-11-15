package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Recorder struct {
	sourceGauge    *prometheus.GaugeVec
	conditionGauge *prometheus.GaugeVec
}

func NewRecorder() *Recorder {
	return &Recorder{
		sourceGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cmmc_resource_sources",
				Help: "Number of sources per resource.",
			},
			[]string{"kind", "namespace", "name"},
		),
		conditionGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cmmc_resource_condition",
				Help: "The current condition of the CMMC Resource.",
			},
			[]string{"kind", "namespace", "name", "type", "status"},
		),
	}
}

func (r *Recorder) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		r.sourceGauge,
		r.conditionGauge,
	}
}

func (r *Recorder) RecordNumSources(o client.Object, count int) {
	r.sourceGauge.With(resourceLables(o)).Set(float64(count))
}

func (r *Recorder) RecordReadyCondition(o hasStatusCondition) {
	condition := o.FindStatusCondition("Ready")
	if condition == nil {
		condition = &metav1.Condition{Type: "Ready", Status: metav1.ConditionUnknown}
	}
	r.RecordCondition(o, *condition)
}

func (r *Recorder) RecordCondition(o client.Object, condition metav1.Condition) {
	for _, status := range []metav1.ConditionStatus{
		metav1.ConditionTrue,
		metav1.ConditionFalse,
		metav1.ConditionUnknown,
	} {
		var (
			value  float64
			status = string(status)
		)
		if string(condition.Status) == status {
			value = 1
		}
		r.conditionGauge.With(resourceLables(o, prometheus.Labels{
			"type":   condition.Type,
			"status": status,
		})).Set(value)
	}
}

func resourceLables(o client.Object, ls ...prometheus.Labels) prometheus.Labels {
	accessor, _ := meta.TypeAccessor(o)
	labels := prometheus.Labels{
		"kind":      accessor.GetKind(),
		"namespace": o.GetNamespace(),
		"name":      o.GetName(),
	}

	for _, l := range ls {
		for k, v := range l {
			labels[k] = v
		}
	}

	return labels
}

type hasStatusCondition interface {
	client.Object
	FindStatusCondition(string) *metav1.Condition
}
