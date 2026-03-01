/*
Copyright 2026 The Kubernetes resource-state-metrics Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package internal

import (
	"context"
	"sync"
	"time"

	"github.com/kubernetes-sigs/resource-state-metrics/pkg/apis/resourcestatemetrics/v1alpha1"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// configure defines behaviours for working with configuration(s).
type configure interface {
	// build builds the given configuration.
	build(ctx context.Context, stores *sync.Map)
}

// configurer knows how to work with RMM configurations.
type configurer struct {
	configuration              v1alpha1.Configuration
	dynamicClientset           dynamic.Interface
	resource                   *v1alpha1.ResourceMetricsMonitor
	celCostLimit               uint64
	celTimeout                 time.Duration
	celEvaluations             *prometheus.CounterVec
	resourceCardinalityDefault int64
	cardinalityWarningRatio    float64
}

// Ensure configurer implements configure.
var _ configure = &configurer{}

// newConfigurer returns a new configurer.
func newConfigurer(
	dynamicClientset dynamic.Interface,
	resource *v1alpha1.ResourceMetricsMonitor,
	celCostLimit uint64,
	celTimeout time.Duration,
	celEvaluations *prometheus.CounterVec,
	resourceCardinalityDefault int64,
	cardinalityWarningRatio float64,
) *configurer {
	return &configurer{
		configuration:              resource.Spec.Configuration,
		dynamicClientset:           dynamicClientset,
		resource:                   resource,
		celCostLimit:               celCostLimit,
		celTimeout:                 celTimeout,
		celEvaluations:             celEvaluations,
		resourceCardinalityDefault: resourceCardinalityDefault,
		cardinalityWarningRatio:    cardinalityWarningRatio,
	}
}

// build constructs the metric stores from the parsed configuration.
func (c *configurer) build(ctx context.Context, stores *sync.Map) {
	builtStores := make([]*StoreType, 0, len(c.configuration.Stores))
	for i := range c.configuration.Stores {
		s := c.buildStoreFromConfig(ctx, &c.configuration.Stores[i])
		builtStores = append(builtStores, s)
	}
	stores.Store(c.resource.GetUID(), builtStores)
}

func (c *configurer) buildStoreFromConfig(ctx context.Context, store *v1alpha1.Store) *StoreType {
	gvkWithR := buildGVKR(store)

	storeCardinalityLimit := store.CardinalityLimit
	if storeCardinalityLimit <= 0 {
		storeCardinalityLimit = c.resourceCardinalityDefault
	}

	// Build FamilyType slice directly from v1alpha1.Family (no conversion middleware)
	families := make([]*FamilyType, len(store.Families))
	for i := range store.Families {
		families[i] = &FamilyType{Family: store.Families[i]}
	}

	return buildStore(
		ctx,
		c.dynamicClientset,
		gvkWithR,
		families,
		store.Selectors.Label, store.Selectors.Field,
		store.Resolver,
		store.Labels,
		c.celCostLimit,
		c.celTimeout,
		c.celEvaluations,
		c.resource.GetNamespace(),
		c.resource.GetName(),
		storeCardinalityLimit,
		c.cardinalityWarningRatio,
	)
}

func buildGVKR(store *v1alpha1.Store) gvkr {
	return gvkr{
		GroupVersionKind: schema.GroupVersionKind{
			Group:   store.Group,
			Version: store.Version,
			Kind:    store.Kind,
		},
		GroupVersionResource: schema.GroupVersionResource{
			Group:    store.Group,
			Version:  store.Version,
			Resource: store.Resource,
		},
	}
}
