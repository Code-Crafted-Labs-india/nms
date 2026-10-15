package handlers

import (
	"reflect"
	"strings"
	"testing"
)

func TestRenderConfigurationCommandsRejectsInvalidRequests(t *testing.T) {
	base := configurationPlanRequest{
		DeviceID:  1,
		Profile:   "ats3900-aos@3.3.1.1",
		Operation: "create_vlan",
		VLANID:    10,
		VLANName:  "users",
	}
	tests := []struct {
		name   string
		mutate func(*configurationPlanRequest)
	}{
		{name: "zero device id", mutate: func(r *configurationPlanRequest) { r.DeviceID = 0 }},
		{name: "negative device id", mutate: func(r *configurationPlanRequest) { r.DeviceID = -1 }},
		{name: "unknown profile", mutate: func(r *configurationPlanRequest) { r.Profile = "unknown-os@1.0" }},
		{name: "VLAN below allowed range", mutate: func(r *configurationPlanRequest) { r.VLANID = 1 }},
		{name: "VLAN above allowed range", mutate: func(r *configurationPlanRequest) { r.VLANID = 4095 }},
		{name: "VLAN name contains whitespace", mutate: func(r *configurationPlanRequest) { r.VLANName = "user vlan" }},
		{name: "VLAN name contains command separator", mutate: func(r *configurationPlanRequest) { r.VLANName = "users;reboot" }},
		{name: "VLAN name contains newline", mutate: func(r *configurationPlanRequest) { r.VLANName = "users\nreboot" }},
		{name: "VLAN name exceeds limit", mutate: func(r *configurationPlanRequest) { r.VLANName = strings.Repeat("a", 33) }},
		{name: "unsupported operation", mutate: func(r *configurationPlanRequest) { r.Operation = "delete_vlan" }},
		{name: "missing interface", mutate: func(r *configurationPlanRequest) { r.Operation, r.Interface = "configure_port", "" }},
		{name: "interface begins with number", mutate: func(r *configurationPlanRequest) { r.Operation, r.Interface = "configure_port", "1/0/1" }},
		{name: "interface contains whitespace", mutate: func(r *configurationPlanRequest) { r.Operation, r.Interface = "configure_port", "g0/0/1 bad" }},
		{name: "interface contains command separator", mutate: func(r *configurationPlanRequest) { r.Operation, r.Interface = "configure_port", "g0/0/1;reboot" }},
		{name: "interface contains newline", mutate: func(r *configurationPlanRequest) { r.Operation, r.Interface = "configure_port", "g0/0/1\nreboot" }},
		{name: "interface exceeds limit", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface = "configure_port", "g"+strings.Repeat("1", 32)
		}},
		{name: "unsupported port mode", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface, r.PortMode = "configure_port", "g0/0/1", "hybrid"
		}},
		{name: "trunk has no allowed VLANs", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface, r.PortMode = "configure_port", "g0/0/1", "trunk"
		}},
		{name: "trunk allowed VLAN below range", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface, r.PortMode, r.AllowedVLANs = "configure_port", "g0/0/1", "trunk", []int{0}
		}},
		{name: "trunk allowed VLAN above range", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface, r.PortMode, r.AllowedVLANs = "configure_port", "g0/0/1", "trunk", []int{4095}
		}},
		{name: "trunk has duplicate VLAN", mutate: func(r *configurationPlanRequest) {
			r.Operation, r.Interface, r.PortMode, r.AllowedVLANs = "configure_port", "g0/0/1", "trunk", []int{10, 10}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := base
			tt.mutate(&request)
			if commands, _, err := renderConfigurationCommands(request); err == nil {
				t.Errorf("expected request to fail, got commands %q", commands)
			}
		})
	}
}

func TestRenderConfigurationCommandsCreatesVLANForEachProfile(t *testing.T) {
	tests := []struct {
		profile  string
		commands []string
	}{
		{
			profile:  "ats3900-aos@3.3.1.1",
			commands: []string{"config", "vlan-view 10", "name users", "quit"},
		},
		{
			profile:  "ats-cli@7.2.1",
			commands: []string{"configure terminal", "vlan 10", "name users", "exit"},
		},
		{
			profile:  "routing-v3.3.1",
			commands: []string{"configure terminal", "vlan database", "vlan 10 name users state enable", "exit"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got, params, err := renderConfigurationCommands(configurationPlanRequest{
				DeviceID: 1, Profile: tt.profile, Operation: "create_vlan", VLANID: 10, VLANName: "users",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.commands) {
				t.Errorf("commands = %q, want %q", got, tt.commands)
			}
			if params["vlan_id"] != 10 || params["vlan_name"] != "users" {
				t.Errorf("unexpected parameters: %#v", params)
			}
		})
	}
}

func TestRenderConfigurationCommandsDefaultsVLANNameAndAcceptsBoundaries(t *testing.T) {
	for _, vlanID := range []int{2, 4094} {
		commands, params, err := renderConfigurationCommands(configurationPlanRequest{
			DeviceID: 1, Profile: "ats3900-aos@3.3.1.1", Operation: "create_vlan", VLANID: vlanID,
		})
		if err != nil {
			t.Fatalf("VLAN ID %d was rejected: %v", vlanID, err)
		}
		if got := params["vlan_name"]; got != "VLAN"+strings.TrimSpace(strings.TrimPrefix(commands[1], "vlan-view ")) {
			t.Errorf("VLAN ID %d default name = %#v", vlanID, got)
		}
	}
	if _, _, err := renderConfigurationCommands(configurationPlanRequest{
		DeviceID: 1, Profile: "ats3900-aos@3.3.1.1", Operation: "create_vlan", VLANID: 10, VLANName: strings.Repeat("a", 32),
	}); err != nil {
		t.Errorf("32-character VLAN name was rejected: %v", err)
	}
}

func TestRenderConfigurationCommandsAccessPorts(t *testing.T) {
	tests := []struct {
		profile  string
		commands []string
	}{
		{profile: "ats3900-aos@3.3.1.1", commands: []string{"config", "interface g0/0/1", "switchport mode access", "switchport pvid 10"}},
		{profile: "ats-cli@7.2.1", commands: []string{"configure terminal", "interface g0/0/1", "switchport mode access", "switchport pvid 10"}},
		{profile: "routing-v3.3.1", commands: []string{"configure terminal", "interface g0/0/1", "switchport mode access", "switchport access vlan 10"}},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got, _, err := renderConfigurationCommands(configurationPlanRequest{
				DeviceID: 1, Profile: tt.profile, Operation: "configure_port", VLANID: 10,
				Interface: "g0/0/1", PortMode: "access",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.commands) {
				t.Errorf("commands = %q, want %q", got, tt.commands)
			}
		})
	}
}

func TestRenderConfigurationCommandsTrunkPorts(t *testing.T) {
	tests := []struct {
		profile  string
		commands []string
	}{
		{profile: "ats3900-aos@3.3.1.1", commands: []string{"config", "interface g0/0/1", "switchport mode trunk", "switchport trunk vlan-allowed 10,20,1"}},
		{profile: "ats-cli@7.2.1", commands: []string{"configure terminal", "interface g0/0/1", "switchport mode trunk", "switchport trunk vlan-allowed 10,20,1"}},
		{profile: "routing-v3.3.1", commands: []string{"configure terminal", "interface g0/0/1", "switchport mode trunk", "switchport trunk allowed vlan add 10,20,1"}},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			got, params, err := renderConfigurationCommands(configurationPlanRequest{
				DeviceID: 1, Profile: tt.profile, Operation: "configure_port", VLANID: 10,
				Interface: "g0/0/1", PortMode: "trunk", AllowedVLANs: []int{10, 20, 1},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.commands) {
				t.Errorf("commands = %q, want %q", got, tt.commands)
			}
			if !reflect.DeepEqual(params["allowed_vlans"], []int{10, 20, 1}) {
				t.Errorf("allowed_vlans parameter = %#v", params["allowed_vlans"])
			}
		})
	}
}
