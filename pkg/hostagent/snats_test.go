// Copyright 2017 Cisco Systems, Inc.
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

package hostagent

import (
	"encoding/json"
	"io/ioutil"
	//"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	//v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"

	"github.com/noironetworks/aci-containers/pkg/metadata"
	//md "github.com/noironetworks/aci-containers/pkg/metadata"
	tu "github.com/noironetworks/aci-containers/pkg/testutil"
	snatv1 "github.com/noironetworks/aci-containers/pkg/snatallocation/apis/aci.snat/v1"
)

type  portRange struct {
	start int
	end   int
}

func snatdata(uuid string, poduuid string, namespace string, name string,
ip string, mac string, port_range portRange, egAnnot string, sgAnnot string) *snatv1.SnatAllocation {
	return &snatv1.SnatAllocation{
		Spec: snatv1.SnatAllocationSpec{
			PodName:  name,
			NodeName: "test-node",
			SnatPortRange: snatv1.PortRange {
				Start:  port_range.start,
				End:   port_range.end,
			},
			SnatIp: ip,
			MacAddress: mac,
			PodUuid: poduuid,
		},
		ObjectMeta: metav1.ObjectMeta{
			UID:       apitypes.UID(uuid),
			Namespace: namespace,
			Name:      name,
			Annotations: map[string]string{
				metadata.EgAnnotation: egAnnot,
				metadata.SgAnnotation: sgAnnot,
			},
			Labels: map[string]string{},
		},
	}
}

type snatTest struct {
	uuid      string
	poduuid   string
	namespace string
	name      string
	ip        string
	mac       string
	port_range portRange
	eg        string
	sg        string
}

var snatTests = []snatTest{
	{
		"730a8e7a-8455-4d46-8e6e-n4fdf0e3a688",
		"730a8e7a-8455-4d46-8e6e-f4fdf0e3a667",
		"testns",
		"pod1",
		"10.1.1.7",
		"00:0c:29:92:fe:d0",
		portRange {3000, 4000},
		egAnnot,
		sgAnnot,
	},
	{
		"730a8e7a-8455-4d46-8e6e-n4fdf0e3a655",
		"6a281ef1-0fcb-4140-a38c-62977ef25d72",
		"testns",
		"pod2",
		"10.1.1.8",
		"00:0c:29:92:fe:d0",
		portRange {4000, 5000},
		egAnnot,
		sgAnnot,
	},
	{
		"730a8e7a-8455-4d46-8e6e-n4fdf0e3a655",
		"6a281ef1-0fcb-4140-a38c-62977ef25d71",
		"testns",
		"pod3",
		"10.1.1.8",
		"00:0c:29:92:fe:d0",
		portRange {4000, 5000},
		egAnnot,
		sgAnnot,
	},
	{
		"730a8e7a-8455-4d46-8e6e-n4fdf0e3a655",
		"6a281ef1-0fcb-4140-a38c-62977ef25d73",
		"testns",
		"pod3",
		"10.1.1.8",
		"00:0c:29:92:fe:d0",
		portRange {9000, 10000},
		egAnnot,
		sgAnnot,
	},
	{
		"730a8e7a-8455-4d46-8e6e-n4fdf0e3a655",
		"6a281ef1-0fcb-4140-a38c-62977ef25d74",
		"testns",
		"pod4",
		"10.1.1.8",
		"00:0c:29:92:fe:d0",
		portRange {9000, 10000},
		egAnnot,
		sgAnnot,
	},
	/*
	{
		"",
		"683c333d-a594-4f00-baa6-0d578a13d83a",
		"testns",
		"pod3",
		"10.1.1.10",
		"52:54:00:e5:26:57",
		portRange {5000, 6000},
		egAnnot,
		sgAnnot,
	},
	*/
}

func (agent *testHostAgent) doTestSnat(t *testing.T, tempdir string,
	pt *snatTest, desc string) {
	var raw []byte
	snat := &OpflexSnatIp{}

	tu.WaitFor(t, pt.name, 100*time.Millisecond,
		func(last bool) (bool, error) {
			var err error
			snatfile := filepath.Join(tempdir,
				pt.uuid + ".snat")
			raw, err = ioutil.ReadFile(snatfile)
			if !tu.WaitNil(t, last, err, desc, pt.name, "read snat") {
				return false, nil
			}
			err = json.Unmarshal(raw, snat)
			agent.log.Info("Snat file added ", snatfile)
			return tu.WaitNil(t, last, err, desc, pt.name, "unmarshal snat"), nil
		})
	agent.log.Info("Snat Object added ", snat)
	snatdstr := pt.uuid 
	assert.Equal(t, snatdstr, snat.Uuid, desc, pt.name, "uuid")
	assert.Equal(t, pt.ip, snat.SnatIp, desc, pt.name, "ip")
}

func TestSnatSync(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "hostagent_test_")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempdir)

	agent := testAgent()
	agent.config.OpFlexSnatDir = tempdir
	agent.config.OpFlexEndpointDir = tempdir
        agent.config.OpFlexServiceDir = tempdir
	agent.config.UplinkIface = "eth10"
        agent.config.ServiceVlan = 4003
	agent.run()
	for i, pt := range podTests {
		if i%2 == 0 {
			ioutil.WriteFile(filepath.Join(tempdir,
				pt.uuid+"_"+pt.cont+"_"+pt.veth+".ep"),
				[]byte("random gibberish"), 0644)
		}

		pod := pod(pt.uuid, pt.namespace, pt.name, pt.eg, pt.sg)
		cnimd := cnimd(pt.namespace, pt.name, pt.ip, pt.cont, pt.veth)
		agent.epMetadata[pt.namespace+"/"+pt.name] =
			map[string]*metadata.ContainerMetadata{
				cnimd.Id.ContId: cnimd,
			}
		agent.fakePodSource.Add(pod)
		time.Sleep(1000 * time.Millisecond)
		//agent.doTestPod(t, tempdir, &pt, "create")
	}
	for i, pt := range snatTests {
		if i%2 == 0 {
			ioutil.WriteFile(filepath.Join(tempdir,
				pt.uuid +".snat"),
				[]byte("random gibberish"), 0644)
		}

		snat := snatdata(pt.uuid, pt.poduuid, pt.namespace, pt.name, pt.ip, pt.mac, pt.port_range, pt.eg, pt.sg)
	/*
		cnimd := cnimd(pt.namespace, pt.name, pt.ip, pt.cont, pt.veth)
		agent.epMetadata[pt.namespace+"/"+pt.name] =
			map[string]*metadata.ContainerMetadata{
				cnimd.Id.ContId: cnimd,
			}
	*/
		agent.fakeSnatSource.Add(snat)
		agent.doTestSnat(t, tempdir, &pt, "create")
	}
	time.Sleep(3000 * time.Millisecond)
	for _, pt := range snatTests {
		snat := snatdata(pt.uuid, pt.poduuid, pt.namespace, pt.name, pt.ip, pt.mac, pt.port_range, pt.eg, pt.sg)
		agent.fakeSnatSource.Add(snat)

		agent.doTestSnat(t, tempdir, &pt, "update")
	}
	time.Sleep(3000 * time.Millisecond)

	for _, pt := range snatTests {
		snat := snatdata(pt.uuid, pt.poduuid, pt.namespace, pt.name, pt.ip, pt.mac, pt.port_range, pt.eg, pt.sg)
		agent.fakeSnatSource.Delete(snat)

		tu.WaitFor(t, pt.name, 100*time.Millisecond,
			func(last bool) (bool, error) {
				snatfile := filepath.Join(tempdir,
					pt.uuid+".snat")
				_, err := ioutil.ReadFile(snatfile)
				return tu.WaitNotNil(t, last, err, "snat deleted"), nil
			})
	}
	agent.stop()
}
