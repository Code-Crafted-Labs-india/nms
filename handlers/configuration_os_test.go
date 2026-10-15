package handlers

import "testing"

func TestDetectConfigurationOS(t *testing.T) {
	tests := []struct {
		name       string
		identity   deviceIdentity
		status     string
		family     string
		version    string
		profile    string
		confidence string
	}{
		{
			name:       "no system description is not identified, even with a model hint",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X"},
			status:     "unidentified",
			confidence: "none",
		},
		{
			name:       "ATS 3900 exact hardware and screenshot OS",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "ATS-3900-24P6X Software Version: AOS 3.3.1.1"},
			status:     "supported",
			family:     "ATS-3900-24P6X · AOS",
			version:    "3.3.1.1",
			profile:    "ats3900-aos@3.3.1.1",
			confidence: "high",
		},
		{
			name:       "ATS 3900 inventory model and AOS without reported model",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "AOS 3.3.1.1"},
			status:     "supported",
			family:     "ATS-3900-24P6X · AOS",
			version:    "3.3.1.1",
			profile:    "ats3900-aos@3.3.1.1",
			confidence: "medium",
		},
		{
			name:       "ATS 3900 version missing",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "ATS-3900-24P6X AOS"},
			status:     "version_unreported",
			family:     "ATS-3900-24P6X · AOS",
			confidence: "medium",
		},
		{
			name:       "ATS 3900 unsupported AOS version",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "ATS-3900-24P6X AOS 3.3.1.2"},
			status:     "unsupported_version",
			family:     "ATS-3900-24P6X · AOS",
			version:    "3.3.1.2",
			confidence: "high",
		},
		{
			name:       "ATS 3900 abbreviated model from sysDescr",
			identity:   deviceIdentity{SysDescr: "ats 3900 24p6x software version aos v3.3.1.1"},
			status:     "supported",
			family:     "ATS-3900-24P6X · AOS",
			version:    "3.3.1.1",
			profile:    "ats3900-aos@3.3.1.1",
			confidence: "high",
		},
		{
			name:       "ATS family release is version qualified",
			identity:   deviceIdentity{SysDescr: "Allied Telesis x930 Software Version 7.2.1"},
			status:     "supported",
			family:     "ATS",
			version:    "7.2.1",
			profile:    "ats-cli@7.2.1",
			confidence: "medium",
		},
		{
			name:       "ATS release suffix is preserved",
			identity:   deviceIdentity{SysDescr: "ATS x930 release 7.2.1+build.4"},
			status:     "supported",
			family:     "ATS",
			version:    "7.2.1+build.4",
			profile:    "ats-cli@7.2.1+build.4",
			confidence: "medium",
		},
		{
			name:       "ATS family missing version",
			identity:   deviceIdentity{SysDescr: "Allied Telesis x930 switch"},
			status:     "version_unreported",
			family:     "ATS",
			confidence: "medium",
		},
		{
			name:       "ATS inventory model is not confirmed by system description",
			identity:   deviceIdentity{DeviceType: "Allied Telesis x930", SysDescr: "Generic switch software version 7.2.1"},
			status:     "identity_unconfirmed",
			family:     "Allied Telesis x930",
			version:    "7.2.1",
			confidence: "low",
		},
		{
			name:       "routing switch reference version",
			identity:   deviceIdentity{SysDescr: "Routing Switch Command Line Reference Issue V3.3.1"},
			status:     "supported",
			family:     "Routing Switch",
			version:    "3.3.1",
			profile:    "routing-v3.3.1",
			confidence: "high",
		},
		{
			name:       "routing switch version missing",
			identity:   deviceIdentity{SysDescr: "Routing Switch OS"},
			status:     "version_unreported",
			family:     "Routing Switch",
			confidence: "medium",
		},
		{
			name:       "routing switch unsupported version",
			identity:   deviceIdentity{SysDescr: "Routing Switch software version 3.3.2"},
			status:     "unsupported_version",
			family:     "Routing Switch",
			version:    "3.3.2",
			confidence: "high",
		},
		{
			name:       "routing switch inventory model is not confirmed by system description",
			identity:   deviceIdentity{DeviceType: "Routing Switch", SysDescr: "Generic switch software version 3.3.1"},
			status:     "identity_unconfirmed",
			family:     "Routing Switch",
			version:    "3.3.1",
			confidence: "low",
		},
		{
			name:       "unrecognized inventory model and system description",
			identity:   deviceIdentity{DeviceType: "NCS 540", SysDescr: "Generic switch software version 3.3.1"},
			status:     "unsupported",
			version:    "3.3.1",
			confidence: "medium",
		},
		{
			name:       "known model without confirming system description",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "Generic embedded network operating system 3.3.1.1"},
			status:     "identity_unconfirmed",
			family:     "ATS-3900-24P6X · AOS",
			confidence: "low",
		},
		{
			name:       "unknown vendor description",
			identity:   deviceIdentity{SysDescr: "ExampleOS release 5.4.2"},
			status:     "unsupported",
			version:    "5.4.2",
			confidence: "medium",
		},
		{
			name:       "five component version is not truncated into a supported version",
			identity:   deviceIdentity{DeviceType: "ATS-3900-24P6X", SysDescr: "ATS-3900-24P6X AOS 3.3.1.1.5"},
			status:     "version_unreported",
			family:     "ATS-3900-24P6X · AOS",
			confidence: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectConfigurationOS(tt.identity)
			if got.Status != tt.status {
				t.Errorf("status = %q, want %q", got.Status, tt.status)
			}
			if got.Family != tt.family {
				t.Errorf("family = %q, want %q", got.Family, tt.family)
			}
			if got.Version != tt.version {
				t.Errorf("version = %q, want %q", got.Version, tt.version)
			}
			if got.Profile != tt.profile {
				t.Errorf("profile = %q, want %q", got.Profile, tt.profile)
			}
			if got.Confidence != tt.confidence {
				t.Errorf("confidence = %q, want %q", got.Confidence, tt.confidence)
			}
			if tt.status != "supported" && got.Profile != "" {
				t.Errorf("unsupported detection selected profile %q", got.Profile)
			}
		})
	}
}

func TestExtractConfigurationVersionSkipsMalformedCandidate(t *testing.T) {
	got := extractConfigurationVersion("Vendor firmware 3.3.1.1.5; compatible AOS 3.3.1.1")
	if got != "3.3.1.1" {
		t.Fatalf("extractConfigurationVersion() = %q, want valid later version", got)
	}
}

func TestDetectConfigurationOSEvidenceAndObservation(t *testing.T) {
	identity := deviceIdentity{
		DeviceType: "ATS-3900-24P6X",
		SysDescr:   "ATS-3900-24P6X AOS 3.3.1.1",
		SysObject:  ".1.3.6.1.4.1.12345.1.99.1234",
		ObservedAt: "2026-10-15T12:00:00Z",
	}
	got := detectConfigurationOS(identity)
	if got.ObservedAt != identity.ObservedAt {
		t.Fatalf("observedAt = %q, want %q", got.ObservedAt, identity.ObservedAt)
	}
	if len(got.Evidence) != 3 {
		t.Fatalf("evidence has %d items, want 3: %#v", len(got.Evidence), got.Evidence)
	}
}

func TestATSConfigurationProfileMatcher(t *testing.T) {
	tests := []struct {
		profile string
		want    bool
	}{
		{profile: "ats-cli", want: true},
		{profile: "ats-cli@3.3.1", want: true},
		{profile: "ATS-CLI@3.3.1+build.4", want: true},
		{profile: "ats-cli@3", want: false},
		{profile: "ats-cli@3.3.1.1.5", want: false},
		{profile: "ats-cli@3.3.1; reboot", want: false},
		{profile: "ats3900-aos@3.3.1.1", want: false},
		{profile: "routing-v3.3.1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			if got := isATSConfigurationProfile(tt.profile); got != tt.want {
				t.Errorf("isATSConfigurationProfile(%q) = %t, want %t", tt.profile, got, tt.want)
			}
		})
	}
	if !isATS3900ConfigurationProfile("ats3900-aos@3.3.1.1") {
		t.Fatal("ATS-3900 AOS profile was not recognized")
	}
	for _, profile := range []string{"ats3900-aos@3.3.1.2", "ATS3900-aos@3.3.1.1", "ats3900-aos@3.3.1.1; reboot"} {
		if isATS3900ConfigurationProfile(profile) {
			t.Errorf("unexpected ATS-3900 profile accepted: %q", profile)
		}
	}
}
