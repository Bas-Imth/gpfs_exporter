// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"os/exec"
	"testing"
)

func TestParseMmcesStateShow(t *testing.T) {
	execCommand = fakeExecCommand
	mockedStdout = `
mmcesstate::HEADER:version:reserved:reserved:NODE:AUTH:BLOCK:NETWORK:AUTH_OBJ:NFS:OBJ:SMB:CES:
mmcesstate::0:1:::ib-protocol01.ten.osc.edu:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:DISABLED:HEALTHY:HEALTHY:
`
	defer func() { execCommand = exec.Command }()
	metrics, err := mmces_state_show_parse(mockedStdout)
	if err != nil {
		t.Errorf("Unexpected error: %s", err.Error())
	}
	if len(metrics) != 8 {
		t.Errorf("Expected 8 metrics returned, got %d", len(metrics))
		return
	}
	if val := metrics[0].Service; val != "AUTH" {
		t.Errorf("Unexpected Service got %s", val)
	}
	if val := metrics[0].State; val != "HEALTHY" {
		t.Errorf("Unexpected State got %s", val)
	}
}

func TestParseMmcesState(t *testing.T) {
	if val := parseMmcesState("HEALTHY"); val != 1 {
		t.Errorf("Expected 1 for HEALTHY, got %v", val)
	}
	if val := parseMmcesState("DISABLED"); val != 0 {
		t.Errorf("Expected 0 for DISABLED, got %v", val)
	}
	if val := parseMmcesState("DEGRADED"); val != 0 {
		t.Errorf("Expected 0 for DEGRADED, got %v", val)
	}
}