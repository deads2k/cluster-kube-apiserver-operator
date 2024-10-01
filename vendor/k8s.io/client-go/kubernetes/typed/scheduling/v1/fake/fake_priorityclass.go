/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"
	json "encoding/json"
	"fmt"

	v1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	schedulingv1 "k8s.io/client-go/applyconfigurations/scheduling/v1"
	testing "k8s.io/client-go/testing"
)

// FakePriorityClasses implements PriorityClassInterface
type FakePriorityClasses struct {
	Fake *FakeSchedulingV1
}

var priorityclassesResource = v1.SchemeGroupVersion.WithResource("priorityclasses")

var priorityclassesKind = v1.SchemeGroupVersion.WithKind("PriorityClass")

// Get takes name of the priorityClass, and returns the corresponding priorityClass object, and an error if there is any.
func (c *FakePriorityClasses) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.PriorityClass, err error) {
	emptyResult := &v1.PriorityClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootGetActionWithOptions(priorityclassesResource, name, options), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PriorityClass), err
}

// List takes label and field selectors, and returns the list of PriorityClasses that match those selectors.
func (c *FakePriorityClasses) List(ctx context.Context, opts metav1.ListOptions) (result *v1.PriorityClassList, err error) {
	emptyResult := &v1.PriorityClassList{}
	obj, err := c.Fake.
		Invokes(testing.NewRootListActionWithOptions(priorityclassesResource, priorityclassesKind, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.PriorityClassList{ListMeta: obj.(*v1.PriorityClassList).ListMeta}
	for _, item := range obj.(*v1.PriorityClassList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested priorityClasses.
func (c *FakePriorityClasses) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchActionWithOptions(priorityclassesResource, opts))
}

// Create takes the representation of a priorityClass and creates it.  Returns the server's representation of the priorityClass, and an error, if there is any.
func (c *FakePriorityClasses) Create(ctx context.Context, priorityClass *v1.PriorityClass, opts metav1.CreateOptions) (result *v1.PriorityClass, err error) {
	emptyResult := &v1.PriorityClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateActionWithOptions(priorityclassesResource, priorityClass, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PriorityClass), err
}

// Update takes the representation of a priorityClass and updates it. Returns the server's representation of the priorityClass, and an error, if there is any.
func (c *FakePriorityClasses) Update(ctx context.Context, priorityClass *v1.PriorityClass, opts metav1.UpdateOptions) (result *v1.PriorityClass, err error) {
	emptyResult := &v1.PriorityClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateActionWithOptions(priorityclassesResource, priorityClass, opts), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PriorityClass), err
}

// Delete takes name of the priorityClass and deletes it. Returns an error if one occurs.
func (c *FakePriorityClasses) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(priorityclassesResource, name, opts), &v1.PriorityClass{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePriorityClasses) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionActionWithOptions(priorityclassesResource, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.PriorityClassList{})
	return err
}

// Patch applies the patch and returns the patched priorityClass.
func (c *FakePriorityClasses) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.PriorityClass, err error) {
	emptyResult := &v1.PriorityClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(priorityclassesResource, name, pt, data, opts, subresources...), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PriorityClass), err
}

// Apply takes the given apply declarative configuration, applies it and returns the applied priorityClass.
func (c *FakePriorityClasses) Apply(ctx context.Context, priorityClass *schedulingv1.PriorityClassApplyConfiguration, opts metav1.ApplyOptions) (result *v1.PriorityClass, err error) {
	if priorityClass == nil {
		return nil, fmt.Errorf("priorityClass provided to Apply must not be nil")
	}
	data, err := json.Marshal(priorityClass)
	if err != nil {
		return nil, err
	}
	name := priorityClass.Name
	if name == nil {
		return nil, fmt.Errorf("priorityClass.Name must be provided to Apply")
	}
	emptyResult := &v1.PriorityClass{}
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceActionWithOptions(priorityclassesResource, *name, types.ApplyPatchType, data, opts.ToPatchOptions()), emptyResult)
	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PriorityClass), err
}
