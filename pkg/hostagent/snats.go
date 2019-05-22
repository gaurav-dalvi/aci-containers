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

// Handlers for pod updates.  Pods map to opflex endpoints

package hostagent

import (
/*
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
*/
	"reflect"
/*
/*
	"strings"
*/
	"github.com/Sirupsen/logrus"
/*
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
*/
	"k8s.io/client-go/tools/cache"

	"k8s.io/kubernetes/pkg/controller"
/*
	"github.com/noironetworks/aci-containers/pkg/metadata"
*/
	//snatv1 "github.com/noironetworks/aci-containers/pkg/snatallocation/clientset/versioned/typed/snat/v1"
	snatv1  "github.com/noironetworks/aci-containers/pkg/snatallocation/apis/snat/v1"
)
type OpflexPortRange struct {
	Start int `json:"port-start,omitempty"`
	End  int `json:"port-end,omitempty"`
}
type OpflexSnatIp struct {
	Uuid string `json:"uuid"`
	IfaceName         string `json:"interface-name,omitempty"`
	IpAddress  string `json:"ip,omitempty"`
	MacAddress string   `json:"mac,omitempty"`
        local bool          `json:"local-enabled"`
	DestIpAddress  string `json:"ip,omitempty"`
	DestPrefix     uint16  `json:"destPrefix,omitempty"`
	PortRange      OpflexPortRange  `json:"port-range,omitempty"`
	InterfaceVlan uint16 `json:"interface-vlan,omitempty"`
	// remote info needs to be papulated
}
/*
func (agent *HostAgent) initPodInformerFromClient(
	kubeClient *kubernetes.Clientset) {

	agent.initPodInformerBase(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector =
					fields.Set{"spec.nodeName": agent.config.NodeName}.String()
				return kubeClient.Core().Pods(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector =
					fields.Set{"spec.nodeName": agent.config.NodeName}.String()
				return kubeClient.Core().Pods(metav1.NamespaceAll).Watch(options)
			},
		})
}
*/
func SnatLogger(log *logrus.Logger,  snat *snatv1.SnatAllocation) *logrus.Entry {
        return log.WithFields(logrus.Fields{
                "namespace": snat.ObjectMeta.Namespace,
                "name":      snat.ObjectMeta.Name,
		"spec":      snat.Spec,
        })
}

func opflexSnatIpLogger(log *logrus.Logger, snatip *OpflexSnatIp) *logrus.Entry {
        return log.WithFields(logrus.Fields{
                "uuid":      snatip.Uuid,
                "snaip":     snatip.IpAddress,
                "start_port": snatip.PortRange.Start,
		"end_port":   snatip.PortRange.End,
        })
}

func (agent *HostAgent) initSnatInformerBase(listWatch *cache.ListWatch) {
	agent.snatInformer = cache.NewSharedIndexInformer(
		listWatch,
		&snatv1.SnatAllocation{},
		controller.NoResyncPeriodFunc(),
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	agent.snatInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			agent.snatUpdate(obj)
		},
		UpdateFunc: func(_ interface{}, obj interface{}) {
			agent.snatUpdate(obj)
		},
		DeleteFunc: func(obj interface{}) {
			agent.snatDelete(obj)
		},
	})
	agent.log.Debug("Initializing Snat Informers")
}

func (agent *HostAgent) snatUpdate(obj interface{}) {
	agent.indexMutex.Lock()
        defer agent.indexMutex.Unlock()
	snat := obj.(*snatv1.SnatAllocation)
	key, err := cache.MetaNamespaceKeyFunc(snat)
        if err != nil {
                SnatLogger(agent.log, snat).
                        Error("Could not create key:" + err.Error())
                return
        }
	agent.log.Info("Snat Object added ", snat)
	agent.doUpdateSnat(key);
}

func (agent *HostAgent) snatDelete(obj interface{}) {
	agent.log.Debug("Snat File sync")
}

func (agent *HostAgent) doUpdateSnat(key string) {
        snatobj, exists, err :=
                agent.snatInformer.GetStore().GetByKey(key)
	if err != nil {
                agent.log.Error("Could not lookup snat for " +
                        key + ": " + err.Error())
                return
        }
        if !exists || snatobj == nil {
                return
        }
	snat  := snatobj.(*snatv1.SnatAllocation)
	logger := SnatLogger(agent.log, snat)
	agent.snatChanged(snatobj, logger)
}

func (agent *HostAgent) snatChanged(snatobj interface{}, logger *logrus.Entry) {
	snat  := snatobj.(*snatv1.SnatAllocation)
	snatUuid := string(snat.ObjectMeta.UID)
	if logger == nil {
		logger = agent.log.WithFields(logrus.Fields{})
	}
	logger.Debug("SnatChanged...")
	snatip := &OpflexSnatIp {
		Uuid: snatUuid,
		IpAddress:  snat.Spec.SnatIp,
	        MacAddress: snat.Spec.MacAddress,
		PortRange: OpflexPortRange { Start: snat.Spec.SnatPortRange.Start,
			     End: snat.Spec.SnatPortRange.End, },
	}
	existing, ok := agent.OpflexSnatIps[snatUuid]
	if (ok && !reflect.DeepEqual(existing, snatip)) || !ok {
		agent.OpflexSnatIps[snatUuid] = snatip
		agent.scheduleSyncSnats()
	}
}

func (agent *HostAgent) syncSnat() bool {
	
	return false;
}
