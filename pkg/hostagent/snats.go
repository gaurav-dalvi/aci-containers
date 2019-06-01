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
// WITHOUT WARRATIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Handlers for pod updates.  Pods map to opflex endpoints

package hostagent

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"github.com/Sirupsen/logrus"
	//v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	//"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"k8s.io/kubernetes/pkg/controller"
	//"github.com/noironetworks/aci-containers/pkg/metadata"
	//snatv1 "github.com/noironetworks/aci-containers/pkg/snatallocation/clientset/versioned/typed/snat/v1"
	snatv1  "github.com/noironetworks/aci-containers/pkg/snatallocation/apis/aci.snat/v1"
	snatclientset "github.com/noironetworks/aci-containers/pkg/snatallocation/clientset/versioned"
)
type OpflexPortRange struct {
	Start int `json:"port-start,omitempty"`
	End  int `json:"port-end,omitempty"`
}

type OpflexSnatIp struct {
	Uuid string `json:"uuid"`
	PodUuids []string `json:"pod-uuid"`
	IfaceName         string `json:"iface-name,omitempty"`
	SnatIp  string `json:"snat-ip,omitempty"`
	MacAddress string   `json:"mac,omitempty"`
        Local bool          `json:"local,omitempty"`
	DestIpAddress  string `json:"destip-dddress,omitempty"`
	DestPrefix     uint16  `json:"destPrefix,omitempty"`
	PortRange      OpflexPortRange  `json:"port-range,omitempty"`
	InterfaceVlan uint `json:"interface_vlan,omitempty"`
	Remote  []OpflexSnatIpRemoteInfo `json:"remote,omitempty"`
}

type OpflexSnatIpRemoteInfo struct {
	NodeIp  string `json:"snat_ip,omitempty"`
	MacAddress string   `json:"mac,omitempty"`
	PortRange      OpflexPortRange  `json:"port-range,omitempty"`
	Refcount int
}
func (agent *HostAgent) initSnatInformerFromClient(
	snatClient *snatclientset.Clientset) {
	agent.initSnatInformerBase(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			//	options.FieldSelector =
			//		fields.Set{"spec.nodeName": agent.config.NodeName}.String()
				return snatClient.AciV1().SnatAllocations(metav1.NamespaceAll).List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			//	options.FieldSelector =
			//		fields.Set{"spec.nodeName": agent.config.NodeName}.String()
				return snatClient.AciV1().SnatAllocations(metav1.NamespaceAll).Watch(options)
			},
		})
}

func getsnat(snatfile string) (string, error) {
	raw, err := ioutil.ReadFile(snatfile)
	if err != nil {
		return "", err
	}
	return string(raw), err
}

func writeSnat(snatfile string, snat *OpflexSnatIp) (bool, error) {
	newdata, err := json.MarshalIndent(snat, "", "  ")
	if err != nil {
		return true, err
	}
	existingdata, err := ioutil.ReadFile(snatfile)
	if err == nil && reflect.DeepEqual(existingdata, newdata) {
		return false, nil
	}

	err = ioutil.WriteFile(snatfile, newdata, 0644)
	return true, err
}

func (agent *HostAgent) FormSnatFilePath(uuid string) string {
	return filepath.Join(agent.config.OpFlexSnatDir, uuid+".snat")
}

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
                "PodUuid":   snatip.PodUuids,
		"snat_ip":     snatip.SnatIp,
		"mac_address": snatip.MacAddress,
                "start_port": snatip.PortRange.Start,
		"end_port":   snatip.PortRange.End,
		"local":      snatip.Local,
		"interface-name": snatip.IfaceName,
		"interfcae-vlan": snatip.InterfaceVlan,
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
	//agent.log.Info("Snat Object added ", snat)
	agent.doUpdateSnat(key);
}

func (agent *HostAgent) snatDelete(obj interface{}) {
	agent.log.Debug("Snat Delete Obj")
	agent.indexMutex.Lock()
        defer agent.indexMutex.Unlock()
	snat := obj.(*snatv1.SnatAllocation)
	snatUuid := string(snat.ObjectMeta.UID)
	agent.snatIpDeleted(&snatUuid)
}

func (agent *HostAgent) snatIpDeleted(snatUuid *string) {
	if _, ok := agent.OpflexSnatIps[*snatUuid]; ok {
		delete(agent.OpflexSnatIps, *snatUuid)
		agent.scheduleSyncSnats()
	}
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
	/*
	epMetaKey := fmt.Sprintf("%s/%s", snat.ObjectMeta.Namespace, snat.ObjectMeta.Name)
	epmetadata, ok := agent.epMetadata[*epMetaKey] 
	*/
	_, ispodlocal := agent.opflexEps[snat.Spec.Poduid]
	existing, ok := agent.OpflexSnatIps[snatUuid]
	remoteinfo := make([]OpflexSnatIpRemoteInfo, 0)
	var snatip *OpflexSnatIp
	podset := false
	if ispodlocal {
		//logger.Debug("Pod is local...")
		poduuids := make([]string, 0)
		if ok {
			for _, uuid := range existing.PodUuids {
				if  uuid == snat.Spec.Poduid {
					podset = true;
				}
				poduuids = append(poduuids, uuid)
			}
			remoteinfo = existing.Remote
		}
		logger.Debug("Pod is local...")

		if podset == false {
			poduuids = append(poduuids, snat.Spec.Poduid)
		}

		agent.log.Debug(poduuids)
		snatip = &OpflexSnatIp {
			Uuid: snatUuid,
			IfaceName: agent.config.UplinkIface,
			MacAddress: snat.Spec.Macaddress,
			SnatIp: snat.Spec.Snatip,
			PodUuids: poduuids,
			Local: ispodlocal,
			PortRange: OpflexPortRange { Start: snat.Spec.Snatportrange.Start,
						    End: snat.Spec.Snatportrange.End, },
			InterfaceVlan: agent.config.ServiceVlan,
			Remote: remoteinfo,
		}
	} else  {
		var remote OpflexSnatIpRemoteInfo
		var macAdress  string
		var snat_ipaddr     string
		poduuids :=  make([]string, 0)
		var portrange OpflexPortRange
		var  local bool
		remote.MacAddress = snat.Spec.Macaddress
		remote.PortRange.Start = snat.Spec.Snatportrange.Start
		remote.PortRange.End = snat.Spec.Snatportrange.End
		if ok {
			remoteinfo = existing.Remote
			macAdress = existing.MacAddress
			snat_ipaddr = existing.SnatIp
			poduuids = existing.PodUuids
			local = existing.Local
			portrange.Start = existing.PortRange.Start
			portrange.End = existing.PortRange.End
		} else {
			snat_ipaddr = snat.Spec.Snatip
			local = false
		}
		agent.log.Debug("existing.Remote", remoteinfo)
		remoteexists := false
		for i, v := range remoteinfo {
			if  v.MacAddress == remote.MacAddress  &&
				v.PortRange.Start == remote.PortRange.Start &&
				v.PortRange.End == remote.PortRange.End {
				remoteinfo[i].Refcount++
				remoteexists = true
			}
		}
		if remoteexists == false {
			remoteinfo = append(remoteinfo, remote)
			remote.Refcount++
		}
		agent.log.Debug("Remote Info", remoteinfo)
		snatip = &OpflexSnatIp {
			Uuid: snatUuid,
			IfaceName: agent.config.UplinkIface,
			MacAddress: macAdress,
			SnatIp: snat_ipaddr,
			PodUuids: poduuids,
			Local: local,
			PortRange:portrange,
			InterfaceVlan: agent.config.ServiceVlan,
			Remote: remoteinfo,
		}
		logger.Debug("Pod is remote...")
	}
	if (ok && !reflect.DeepEqual(existing, snatip)) || !ok {
		agent.OpflexSnatIps[snatUuid] = snatip
		agent.scheduleSyncSnats()
	}
}

func (agent *HostAgent) syncSnat() bool {
	if !agent.syncEnabled {
		return false
	}

	agent.log.Debug("Syncing snats")
	agent.indexMutex.Lock()
	opflexSnatIps := make(map[string]*OpflexSnatIp)
	for k, v := range agent.OpflexSnatIps {
		opflexSnatIps[k] = v
	}
	agent.indexMutex.Unlock()
	files, err := ioutil.ReadDir(agent.config.OpFlexSnatDir)
	if err != nil {
		agent.log.WithFields(
			logrus.Fields{"SnatDir": agent.config.OpFlexSnatDir},
		).Error("Could not read directory " + err.Error())
		return true
	}
	seen := make(map[string]bool)
	for _, f := range files {
		uuid := f.Name()
		if strings.HasSuffix(uuid, ".snat") {
			uuid = uuid[:len(uuid)-5]
		} else {
			continue
		}

		snatfile := filepath.Join(agent.config.OpFlexSnatDir, f.Name())
		logger := agent.log.WithFields(
			logrus.Fields{"Uuid": uuid },)
		existing, ok := opflexSnatIps[uuid]
		if ok {
			fmt.Printf("snatfile:%s\n", snatfile)
			wrote, err := writeSnat(snatfile, existing)
			if err != nil {
				opflexSnatIpLogger(agent.log, existing).Error("Error writing snat file: ", err)
			} else if wrote {
				opflexSnatIpLogger(agent.log, existing).Info("Updated snat")
			}
			for _, poduuid := range opflexSnatIps[uuid].PodUuids {
				agent.UpdateEpFile(poduuid, existing.SnatIp)
			}
			seen[uuid] = true
		} else {
			logger.Info("Removing snat")
			os.Remove(snatfile)
		}
	}
	for _, snat := range opflexSnatIps {
		if seen[snat.Uuid] {
			continue
		}

		opflexSnatIpLogger(agent.log, snat).Info("Adding Snat")
		snatfile :=
			agent.FormSnatFilePath(snat.Uuid)
		_, err = writeSnat(snatfile, snat)
		if err != nil {
			opflexSnatIpLogger(agent.log, snat).
				Error("Error writing snat file: ", err)
		}
		for _, poduuid := range snat.PodUuids {
			agent.UpdateEpFile(poduuid, snat.SnatIp)
		}
	}
	agent.log.Debug("Finished snat sync")
	return false;
}
func (agent *HostAgent) UpdateEpFile(uuid string, snatip string) bool {

	agent.log.Debug("Syncing endpoints")
	agent.indexMutex.Lock()
	opflexEps := make(map[string][]*opflexEndpoint)
	for k, v := range agent.opflexEps {
		opflexEps[k] = v
	}
	agent.indexMutex.Unlock()
	files, err := ioutil.ReadDir(agent.config.OpFlexEndpointDir)
	if err != nil {
		agent.log.WithFields(
			logrus.Fields{"endpointDir": agent.config.OpFlexEndpointDir},
		).Error("Could not read directory ", err)
		return true
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".ep") {
			continue
		}
		epfile := filepath.Join(agent.config.OpFlexEndpointDir, f.Name())
		epidstr := f.Name()
		epidstr = epidstr[:len(epidstr)-3]
		epid := strings.Split(epidstr, "_")

		if len(epid) < 3 {
			agent.log.Warn("Removing invalid endpoint:", f.Name())
			os.Remove(epfile)
			continue
		}
		poduuid := epid[0]
		agent.log.Debug(uuid)
		agent.log.Debug(poduuid)
		if uuid != poduuid {
			continue
		}
		existing, ok := opflexEps[poduuid]
		if ok {
			ok = false
			for i, ep := range existing {
				if ep.Uuid != epidstr {
					continue
				}
				agent.opflexEps[poduuid][i].SnatIp = snatip
				ep.SnatIp = snatip
				fmt.Printf("ep file updated with:%s\n", ep.SnatIp)
				wrote, err := writeEp(epfile, ep)
				if err != nil {
					opflexEpLogger(agent.log, ep).
						Error("Error writing EP file: ", err)
				} else if wrote {
					opflexEpLogger(agent.log, ep).
						Info("Updated endpoint")
				}
				fmt.Printf("ep file updated with:%s\n", epfile)
				ok = true
			}
		}
	}
	agent.log.Debug("Finished endpoint snatip sync")
	return false
}
