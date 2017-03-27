// Copyright 2016 Cisco Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/controller"

	"github.com/noironetworks/aci-containers/pkg/metadata"
)

func (cont *AciController) initEndpointsInformerFromClient(
	kubeClient kubernetes.Interface) {

	cont.initEndpointsInformerBase(&cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return kubeClient.CoreV1().Endpoints(metav1.NamespaceAll).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return kubeClient.CoreV1().Endpoints(metav1.NamespaceAll).Watch(options)
		},
	})
}

func (cont *AciController) initEndpointsInformerBase(listWatch *cache.ListWatch) {
	cont.endpointsInformer = cache.NewSharedIndexInformer(
		listWatch,
		&v1.Endpoints{},
		controller.NoResyncPeriodFunc(),
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	cont.endpointsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cont.endpointsChanged(obj)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			cont.endpointsChanged(obj)
		},
		DeleteFunc: func(obj interface{}) {
			cont.endpointsChanged(obj)
		},
	})

}

func (cont *AciController) initServiceInformerFromClient(
	kubeClient *kubernetes.Clientset) {

	cont.initServiceInformerBase(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return kubeClient.CoreV1().Services(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return kubeClient.CoreV1().Services(metav1.NamespaceAll).Watch(options)
			},
		})
}

func (cont *AciController) initServiceInformerBase(listWatch *cache.ListWatch) {
	cont.serviceInformer = cache.NewSharedIndexInformer(
		listWatch,
		&v1.Service{},
		controller.NoResyncPeriodFunc(),
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	cont.serviceInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cont.serviceChanged(obj)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			cont.serviceChanged(obj)
		},
		DeleteFunc: func(obj interface{}) {
			cont.serviceDeleted(obj)
		},
	})
}

func serviceLogger(log *logrus.Logger, as *v1.Service) *logrus.Entry {
	return log.WithFields(logrus.Fields{
		"namespace": as.ObjectMeta.Namespace,
		"name":      as.ObjectMeta.Name,
		"type":      as.Spec.Type,
	})
}

func (cont *AciController) endpointsChanged(obj interface{}) {
	servicekey, err := cache.MetaNamespaceKeyFunc(obj.(*v1.Endpoints))
	if err != nil {
		cont.log.Error("Could not create service key: ", err)
		return
	}
	cont.queueServiceUpdateByKey(servicekey)
}

func returnIps(pool *netIps, ips []net.IP) {
	for _, ip := range ips {
		if ip.To4() != nil {
			pool.V4.AddIp(ip)
		} else if ip.To16() != nil {
			pool.V6.AddIp(ip)
		}
	}
}

func (cont *AciController) aciNameFromServiceKey(key string) string {
	return "service_" + cont.config.AciPrefix +
		"_" + strings.Replace(key, "/", "_", -1)
}

func (cont *AciController) updateMonitoredExternalNetworks() {
	var aciObjs aciSlice

	cache.ListAllByNamespace(cont.aimInformer.GetIndexer(),
		aimNamespace, labels.SelectorFromSet(labels.Set{
			"aim_type": "external_network",
		}),
		func(aimobj interface{}) {
			aci := aimobj.(*Aci)
			if aci.Spec.Type == "external_network" &&
				aci.Spec.ExternalNetwork != nil &&
				aci.Spec.ExternalNetwork.Monitored == true {
				aciObjs = append(aciObjs, aci)
			}
		})

	cont.reconcileMonitoredExternalNetworks(aciObjs)
}

func (cont *AciController) reconcileMonitoredExternalNetworks(aciObjs aciSlice) {
	enets := make(map[string]bool)
	for _, extNet := range cont.config.AciExtNetworks {
		enets[extNet] = true
	}

	var extNets aciSlice
	for _, aci := range aciObjs {
		_, isEnet := enets[aci.Spec.ExternalNetwork.Name]
		if aci.Spec.ExternalNetwork.TenantName == cont.config.AciL3OutTenant &&
			isEnet {
			extNets = append(extNets, aci)
		}
	}
	if len(extNets) == 0 {
		return
	}

	var cnames []string
	cache.ListAll(cont.serviceInformer.GetIndexer(), labels.Everything(),
		func(serviceobj interface{}) {
			service := serviceobj.(*v1.Service)

			isLoadBalancer := service.Spec.Type == v1.ServiceTypeLoadBalancer
			if !isLoadBalancer {
				return
			}

			servicekey, err := cache.MetaNamespaceKeyFunc(service)
			if err != nil {
				serviceLogger(cont.log, service).
					Error("Could not create service key: ", err)
				return
			}

			cnames = append(cnames, cont.aciNameFromServiceKey(servicekey))
		})

	sort.Strings(cnames)

	for _, aci := range extNets {
		sort.Strings(aci.Spec.ExternalNetwork.ConsumedContractNames)
		if reflect.DeepEqual(cnames,
			aci.Spec.ExternalNetwork.ConsumedContractNames) {
			continue
		}

		aci.Spec.ExternalNetwork.ConsumedContractNames = cnames
		cont.aciObjLogger(aci).Debug("Updating monitored external network: ",
			cnames)
		// Note we don't use AIM index here since this is a monitored
		// object with special behavior in AID
		_, err := cont.updateAim(aci)
		if err != nil {
			cont.log.Error("Could not update AIM object: ", err)
		}
	}
}

func (cont *AciController) staticServiceObjs() aciSlice {
	var serviceObjs aciSlice

	// Service bridge domain
	{
		bd := NewBridgeDomain(cont.config.AciL3OutTenant,
			"kubernetes-service-bd")
		bd.Spec.BridgeDomain.EnableArpFlood = true
		bd.Spec.BridgeDomain.EnableRouting = true
		bd.Spec.BridgeDomain.L2UnknownUnicastMode = "flood"
		// XXX TODO may need to set endpoint dataplane learning to
		// false, but no field currently
		bd.Spec.BridgeDomain.VrfName = cont.config.AciVrf
		serviceObjs = append(serviceObjs, bd)
	}
	for _, cidr := range cont.config.NodeServiceSubnets {
		serviceObjs = append(serviceObjs,
			NewSubnet(cont.config.AciL3OutTenant,
				"kubernetes-service-bd", cidr))

	}

	return serviceObjs
}

func (cont *AciController) initStaticServiceObjs() {
	cont.writeAimObjects("StaticService", "static",
		cont.staticServiceObjs())
}

// can be called with index lock
func (cont *AciController) updateServicesForNode(nodename string) {
	cache.ListAll(cont.endpointsInformer.GetIndexer(), labels.Everything(),
		func(endpointsobj interface{}) {
			endpoints := endpointsobj.(*v1.Endpoints)
			for _, subset := range endpoints.Subsets {
				for _, addr := range subset.Addresses {
					if addr.NodeName != nil && *addr.NodeName == nodename {

						servicekey, err :=
							cache.MetaNamespaceKeyFunc(endpointsobj.(*v1.Endpoints))
						if err != nil {
							cont.log.Error("Could not create endpoints key: ", err)
							return
						}
						cont.queueServiceUpdateByKey(servicekey)
						return
					}
				}
			}
		})
}

func (cont *AciController) updateServiceGraph(key string, service *v1.Service) {
	endpointsobj, exists, err :=
		cont.endpointsInformer.GetStore().GetByKey(key)
	if err != nil {
		cont.log.Error("Could not lookup endpoints for " +
			key + ": " + err.Error())
		return
	}

	cont.indexMutex.Lock()
	nodeMap := make(map[string]metadata.ServiceEndpoint)

	if exists && endpointsobj != nil {
		endpoints := endpointsobj.(*v1.Endpoints)
		for _, subset := range endpoints.Subsets {
			for _, addr := range subset.Addresses {
				if addr.NodeName != nil {
					if nodeMeta, ok := cont.
						nodeServiceMetaCache[*addr.NodeName]; ok {

						nodeMap[*addr.NodeName] = nodeMeta.serviceEp
					}
				}
			}
		}
	}
	cont.indexMutex.Unlock()

	var nodes []string
	for node, _ := range nodeMap {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)

	name := cont.aciNameFromServiceKey(key)
	var serviceObjs aciSlice
	if len(nodes) > 0 {
		// 1. Device cluster:
		// The device cluster is a set of physical paths that need
		// to be created for each unique set of nodes that host
		// services.  It’s also possible to simply configure this
		// with every node and rely on the redirect policy later
		// limit the scope of service redirects.
		{
			dc := NewDeviceCluster(cont.config.AciL3OutTenant, name)
			dc.Spec.DeviceCluster.Managed = false
			dc.Spec.DeviceCluster.PhysicalDomainName =
				cont.config.AciServicePhysDom
			dc.Spec.DeviceCluster.Encap = cont.config.AciServiceEncap
			for _, node := range nodes {
				dc.Spec.DeviceCluster.Devices =
					append(dc.Spec.DeviceCluster.Devices, Devices{
						Name: node,
						// XXX TODO: device path for node
					})
			}
			serviceObjs = append(serviceObjs, dc)
		}

		// 2. Service graph template
		// The service graph controls how the traffic will be
		// redirected.  The service graph should always be created
		// exactly as in the example below.  A service graph must
		// be created for each device cluster.
		// XXX can we reuse the same graph for all services?
		{
			sg := NewServiceGraph(cont.config.AciL3OutTenant, name)
			sg.Spec.ServiceGraph.LinearChainNodes = []LinearChainNodes{
				LinearChainNodes{
					DeviceClusterTenantName: cont.config.AciL3OutTenant,
					DeviceClusterName:       name,
					Name:                    "LoadBalancer",
				},
			}
			serviceObjs = append(serviceObjs, sg)
		}

		// 3. Service redirect policy
		// The service redirect policy contains the MAC address
		// and IP address of each of the service endpoints for
		// each node that hosts a pod for this service.  The
		// example below shows the case of two nodes.
		{
			rp := NewServiceRedirectPolicy(cont.config.AciL3OutTenant, name)
			for _, node := range nodes {
				if serviceEp, ok := nodeMap[node]; ok {
					if serviceEp.Ipv4 != nil {
						rp.Spec.ServiceRedirectPolicy.Destinations =
							append(rp.Spec.ServiceRedirectPolicy.Destinations,
								Destinations{
									Ip:  serviceEp.Ipv4.String(),
									Mac: serviceEp.Mac,
								})
					}
					if serviceEp.Ipv6 != nil {
						rp.Spec.ServiceRedirectPolicy.Destinations =
							append(rp.Spec.ServiceRedirectPolicy.Destinations,
								Destinations{
									Ip:  serviceEp.Ipv6.String(),
									Mac: serviceEp.Mac,
								})
					}
				}
			}
			serviceObjs = append(serviceObjs, rp)
		}

		// 4. Service graph contract
		// The service graph contract must be bound to the service
		// graph.  This contract must be consumed by the default
		// layer 3 network and provided by the service layer 3
		// network.
		{
			en := NewExternalNetwork(cont.config.AciL3OutTenant,
				cont.config.AciL3Out, name)
			en.Spec.ExternalNetwork.ProvidedContractNames =
				[]string{name}
			serviceObjs = append(serviceObjs, en)
		}

		for _, ingress := range service.Status.LoadBalancer.Ingress {
			serviceObjs = append(serviceObjs,
				NewExternalSubnet(cont.config.AciL3OutTenant,
					cont.config.AciL3Out, name, ingress.IP+"/32"))
		}

		{
			serviceObjs = append(serviceObjs,
				NewContract(cont.config.AciL3OutTenant, name))
			cs := NewContractSubject(cont.config.AciL3OutTenant, name,
				"LoadBalancedService")

			fname_in := name + "_in"
			fname_out := name + "_out"
			serviceObjs = append(serviceObjs,
				NewFilter(cont.config.AciL3OutTenant, fname_in))
			serviceObjs = append(serviceObjs,
				NewFilter(cont.config.AciL3OutTenant, fname_out))
			cs.Spec.ContractSubject.InFilters = []string{fname_in}
			cs.Spec.ContractSubject.OutFilters = []string{fname_out}
			serviceObjs = append(serviceObjs, cs)

			for i, port := range service.Spec.Ports {
				fe_in := NewFilterEntry(cont.config.AciL3OutTenant,
					fname_in, strconv.Itoa(i))
				fe_out := NewFilterEntry(cont.config.AciL3OutTenant,
					fname_out, strconv.Itoa(i))

				fe_in.Spec.FilterEntry.EtherType = "ipv4"
				fe_out.Spec.FilterEntry.EtherType = "ipv4"
				if port.Protocol == v1.ProtocolUDP {
					fe_in.Spec.FilterEntry.IpProtocol = "udp"
					fe_out.Spec.FilterEntry.IpProtocol = "udp"
				} else {
					fe_in.Spec.FilterEntry.IpProtocol = "tcp"
					fe_out.Spec.FilterEntry.IpProtocol = "tcp"
				}
				fe_in.Spec.FilterEntry.DestFromPort =
					strconv.Itoa(int(port.Port))
				fe_out.Spec.FilterEntry.SourceFromPort =
					strconv.Itoa(int(port.Port))

				serviceObjs = append(serviceObjs, fe_in)
				serviceObjs = append(serviceObjs, fe_out)
			}

		}

		// 5. Device cluster context
		// The logical device context binds the service contract
		// to the redirect policy and the device cluster and
		// bridge domain for the device cluster.
		{
			cc := NewDeviceClusterContext(cont.config.AciL3OutTenant,
				name, name, "LoadBalancer")
			cc.Spec.DeviceClusterContext.BridgeDomainTenantName =
				cont.config.AciL3OutTenant
			cc.Spec.DeviceClusterContext.BridgeDomainName =
				"kubernetes-service-bd"
			cc.Spec.DeviceClusterContext.DeviceClusterTenantName =
				cont.config.AciL3OutTenant
			cc.Spec.DeviceClusterContext.DeviceClusterName = name
			cc.Spec.DeviceClusterContext.ServiceRedirectPolicyTenantName =
				cont.config.AciL3OutTenant
			cc.Spec.DeviceClusterContext.ServiceRedirectPolicyName = name

			serviceObjs = append(serviceObjs, cc)
		}
	}

	cont.writeAimObjects("Service", name, serviceObjs)
	cont.updateMonitoredExternalNetworks()
}

func (cont *AciController) queueServiceUpdateByKey(key string) {
	cont.serviceQueue.Add(key)
}

func (cont *AciController) queueServiceUpdate(service *v1.Service) {
	key, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		serviceLogger(cont.log, service).
			Error("Could not create service key: ", err)
		return
	}
	cont.serviceQueue.Add(key)
}

func (cont *AciController) serviceChanged(obj interface{}) {
	cont.queueServiceUpdate(obj.(*v1.Service))
}

func (cont *AciController) handleServiceUpdate(service *v1.Service) bool {
	cont.indexMutex.Lock()
	logger := serviceLogger(cont.log, service)

	servicekey, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		logger.Error("Could not create service key: ", err)
		cont.indexMutex.Unlock()
		return false
	}
	meta, ok := cont.serviceMetaCache[servicekey]
	isLoadBalancer := service.Spec.Type == v1.ServiceTypeLoadBalancer
	if ok && !isLoadBalancer {
		cont.indexMutex.Unlock()
		cont.serviceDeleted(service)
		return false
	}
	if !isLoadBalancer {
		cont.indexMutex.Unlock()
		return false
	}
	if !ok {
		meta = &serviceMeta{}
		cont.serviceMetaCache[servicekey] = meta

		// Read any existing IPs and attempt to allocate them to the pod
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			ip := net.ParseIP(ingress.IP)
			if ip == nil {
				continue
			}
			if ip.To4() != nil {
				if cont.serviceIps.V4.RemoveIp(ip) {
					meta.ingressIps = append(meta.ingressIps, ip)
				} else if cont.staticServiceIps.V4.RemoveIp(ip) {
					meta.staticIngressIps = append(meta.staticIngressIps, ip)
				}
			} else if ip.To16() != nil {
				if cont.serviceIps.V6.RemoveIp(ip) {
					meta.ingressIps = append(meta.ingressIps, ip)
				} else if cont.staticServiceIps.V6.RemoveIp(ip) {
					meta.staticIngressIps = append(meta.staticIngressIps, ip)
				}
			}
		}
	}

	// try to give the requested load balancer IP to the pod
	requestedIp := net.ParseIP(service.Spec.LoadBalancerIP)
	if requestedIp != nil {
		hasRequestedIp := false
		for _, ip := range meta.ingressIps {
			if reflect.DeepEqual(requestedIp, ip) {
				hasRequestedIp = true
			}
		}
		if !hasRequestedIp {
			if requestedIp.To4() != nil &&
				cont.staticServiceIps.V4.RemoveIp(requestedIp) {
				hasRequestedIp = true
			} else if requestedIp.To16() != nil &&
				cont.staticServiceIps.V6.RemoveIp(requestedIp) {
				hasRequestedIp = true
			}
		}
		if hasRequestedIp {
			returnIps(cont.serviceIps, meta.ingressIps)
			meta.ingressIps = nil
			meta.staticIngressIps = []net.IP{requestedIp}
			meta.requestedIp = requestedIp
		}
	} else if meta.requestedIp != nil {
		meta.requestedIp = nil
		returnIps(cont.staticServiceIps, meta.staticIngressIps)
		meta.staticIngressIps = nil
	}

	if len(meta.ingressIps) == 0 && len(meta.staticIngressIps) == 0 {
		ipv4, err := cont.serviceIps.V4.GetIp()
		if err != nil {
			logger.Error("No IP addresses available for service")
		} else {
			meta.ingressIps = []net.IP{ipv4}
		}
	}

	cont.indexMutex.Unlock()

	var newIngress []v1.LoadBalancerIngress
	for _, ip := range meta.ingressIps {
		newIngress = append(newIngress, v1.LoadBalancerIngress{IP: ip.String()})
	}
	for _, ip := range meta.staticIngressIps {
		newIngress = append(newIngress, v1.LoadBalancerIngress{IP: ip.String()})
	}

	if !reflect.DeepEqual(newIngress, service.Status.LoadBalancer.Ingress) {
		service.Status.LoadBalancer.Ingress = newIngress

		_, err := cont.updateServiceStatus(service)
		if err != nil {
			logger.Error("Failed to update service: ", err)
		} else {
			logger.WithFields(logrus.Fields{
				"status": service.Status.LoadBalancer.Ingress,
			}).Info("Updated service load balancer status")
		}
	}

	cont.updateServiceGraph(servicekey, service)
	return false
}

func (cont *AciController) serviceDeleted(obj interface{}) {
	service := obj.(*v1.Service)
	logger := serviceLogger(cont.log, service)

	servicekey, err := cache.MetaNamespaceKeyFunc(service)
	if err != nil {
		logger.Error("Could not create service key: ", err)
		return
	}
	cont.indexMutex.Lock()
	if meta, ok := cont.serviceMetaCache[servicekey]; ok {
		returnIps(cont.serviceIps, meta.ingressIps)
		returnIps(cont.staticServiceIps, meta.staticIngressIps)
		delete(cont.serviceMetaCache, servicekey)
	}
	cont.indexMutex.Unlock()
	cont.clearAimObjects("Service", cont.aciNameFromServiceKey(servicekey))
	cont.updateMonitoredExternalNetworks()
}