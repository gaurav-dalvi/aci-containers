/***
Copyright 2019 Cisco Systems Inc. All rights reserved.

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

package v1

import (
	"time"

	v1 "github.com/noironetworks/aci-containers/pkg/snatallocation/apis/aci.snat/v1"
	scheme "github.com/noironetworks/aci-containers/pkg/snatallocation/clientset/versioned/scheme"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SnatAllocationsGetter has a method to return a SnatAllocationInterface.
// A group's client should implement this interface.
type SnatAllocationsGetter interface {
	SnatAllocations(namespace string) SnatAllocationInterface
}

// SnatAllocationInterface has methods to work with SnatAllocation resources.
type SnatAllocationInterface interface {
	Create(*v1.SnatAllocation) (*v1.SnatAllocation, error)
	Update(*v1.SnatAllocation) (*v1.SnatAllocation, error)
	Delete(name string, options *metav1.DeleteOptions) error
	DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error
	Get(name string, options metav1.GetOptions) (*v1.SnatAllocation, error)
	List(opts metav1.ListOptions) (*v1.SnatAllocationList, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SnatAllocation, err error)
	SnatAllocationExpansion
}

// snatAllocations implements SnatAllocationInterface
type snatAllocations struct {
	client rest.Interface
	ns     string
}

// newSnatAllocations returns a SnatAllocations
func newSnatAllocations(c *SnatV1Client, namespace string) *snatAllocations {
	return &snatAllocations{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the snatAllocation, and returns the corresponding snatAllocation object, and an error if there is any.
func (c *snatAllocations) Get(name string, options metav1.GetOptions) (result *v1.SnatAllocation, err error) {
	result = &v1.SnatAllocation{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("snatallocations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SnatAllocations that match those selectors.
func (c *snatAllocations) List(opts metav1.ListOptions) (result *v1.SnatAllocationList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.SnatAllocationList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("snatallocations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested snatAllocations.
func (c *snatAllocations) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("snatallocations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a snatAllocation and creates it.  Returns the server's representation of the snatAllocation, and an error, if there is any.
func (c *snatAllocations) Create(snatAllocation *v1.SnatAllocation) (result *v1.SnatAllocation, err error) {
	result = &v1.SnatAllocation{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("snatallocations").
		Body(snatAllocation).
		Do().
		Into(result)
	return
}

// Update takes the representation of a snatAllocation and updates it. Returns the server's representation of the snatAllocation, and an error, if there is any.
func (c *snatAllocations) Update(snatAllocation *v1.SnatAllocation) (result *v1.SnatAllocation, err error) {
	result = &v1.SnatAllocation{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("snatallocations").
		Name(snatAllocation.Name).
		Body(snatAllocation).
		Do().
		Into(result)
	return
}

// Delete takes name of the snatAllocation and deletes it. Returns an error if one occurs.
func (c *snatAllocations) Delete(name string, options *metav1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("snatallocations").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *snatAllocations) DeleteCollection(options *metav1.DeleteOptions, listOptions metav1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("snatallocations").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched snatAllocation.
func (c *snatAllocations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.SnatAllocation, err error) {
	result = &v1.SnatAllocation{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("snatallocations").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
