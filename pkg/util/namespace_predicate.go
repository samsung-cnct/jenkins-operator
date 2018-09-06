package util

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"strings"
)

type NamespacedResourceVersionChangedPredicate struct {
	namespaces       []string
	defaultPredicate predicate.Predicate
}

func (n NamespacedResourceVersionChangedPredicate) Create(event event.CreateEvent) bool {
	return n.contains(event.Meta.GetNamespace()) && n.defaultPredicate.Create(event)
}

func (n NamespacedResourceVersionChangedPredicate) Delete(event event.DeleteEvent) bool {
	return n.contains(event.Meta.GetNamespace()) && n.defaultPredicate.Delete(event)
}

func (n NamespacedResourceVersionChangedPredicate) Update(event event.UpdateEvent) bool {
	return n.contains(event.MetaNew.GetNamespace()) && n.defaultPredicate.Update(event)
}

func (n NamespacedResourceVersionChangedPredicate) Generic(event event.GenericEvent) bool {
	return n.contains(event.Meta.GetNamespace()) && n.defaultPredicate.Generic(event)
}

func (n NamespacedResourceVersionChangedPredicate) contains(subject string) bool {
	for _, item := range n.namespaces {
		if subject == item {
			return true
		}
	}
	return false
}

// newPredicate creates a namespace-based predicate for the Watch functions
// or returns a default one if no namespace was specified
func NewPredicate(namespaces string) predicate.Predicate {
	if namespaces == "" {
		return predicate.ResourceVersionChangedPredicate{}
	}

	return NamespacedResourceVersionChangedPredicate{
		namespaces:       strings.Split(namespaces, ","),
		defaultPredicate: predicate.ResourceVersionChangedPredicate{},
	}
}
