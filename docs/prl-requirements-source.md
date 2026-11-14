# PRL NMS requirements — source transcription

This file records the requirement text supplied in the project conversation on
2026-11-14. Formatting has been normalized into Markdown; wording, numbering,
and requirement coverage are retained. The supplied text ends mid-word at the
end of clause 13, so the final source sentence is incomplete and must be
checked against the original PRL document before a bid or compliance response.
The compliance analysis is in
[`client-requirements-gap-plan.md`](client-requirements-gap-plan.md).

| Sr. No. | Specification | Compliance (Yes/No) | Remarks |
|---|---|---|---|
| 1 | The provided Network Management Software solution should be preferably from same OEM as provided switches or well established off-the-shelf NMS solutions only. The NMS should be able to configure switch specific all types of configurations. It has to Configure and manage all the configurable parameters’ of the proposed switch. |  |  |
| 2 | The NMS software shall be compatible with RHEL. PRL will provide required resources in virtual environment. The bidder must provide the required hardware details in the Remarks column of this point. |  |  |
| 3 | The proposed NMS should support a unified monitoring platform It should provide an auditing system to record for post-audit. |  |  |
| 4 | NMS must allow IT administrators to easily define a number of pre-configured network policies, and designate select personnel to activate/deactivate these policies as appropriate. To achieve this It should have different user roles and privileges. |  |  |
| 5 | The management software should be capable of displaying all-round network topology information to facilitate routine operation and maintenance. Administrator should be able to view traffic, device performance, and access terminals based on the topologies. |  |  |
| 6 | The management software should allow highlighting important devices; mark important links; view information such as interface sending/receiving rate and interface inbound/outbound bandwidth usage; and set different colors for links |  |  |
| 7 | NMS should be capable of managing all the switches deployed across multi-campus vide deployed Local Area Network of PRL.<br>i.e PRL has other three campuses connected to Main campus via Managed Leased Line Network (MLLN) from external service provider, the NMS should detect, display managed entirely all the switches deployed in all the campuses. |  |  |
| 8 | NMS must provide a system wide deployment of VLAN configuration and monitoring capabilities. |  |  |
| 9 | Alarm management |  |  |
| 9a | The management software should support real-time monitoring on alarms of all devices on the entire network and notification of alarms in various manners, for example, display on the topology page, SNMP trap, Syslog, Custom script and emails. It should support customization of the content in emails. |  |  |
| 10 | Performance and Inventory management |  |  |
| 10a | The management software should support task-based and long-term performance monitoring. It should generate alarms of different levels (critical, major, minor, and warning) based on the configured performance threshold values |  |  |
| 10b | Must be able to define policies for the following:<br>To rate-limit bandwidth<br>To throttle the rate of new network connections<br>To prioritize based on Layer 2 or Layer 3 QoS mechanisms<br>To apply packet tags<br>To isolate/quarantine a particular port or VLAN<br>To trigger pre-defined actions. |  |  |
| 10c | Should be able to track device attributes such as asset tags, serial numbers, CPU type, firmware version and memory of all the devices being managed |  |  |
| 10d | Should be able to provide/present detailed configuration information such as date and time when configuration was saved, file size and firmware version etc. |  |  |
| 10e | Should be able to record history of all the devices/switches attributes and be able to report in case if any changes are made to the device. |  |  |
| 10f | Should be capable to provide history of configuration changes made and the firmware of the device. |  |  |
| 10g | Should be capable of generating in-depth reports for the purpose of network inventory planning. |  |  |
| 10h | There should be support for downloading the firmware(s) of single/multiple devices simultaneously |  |  |
| 10i | Must provide port level analysis capability |  |  |
| 11 | Unified network |  |  |
| 11a | The NMS should support a unified management system for uniformly managing network switches. |  |  |
| 11b | The management functions should include security policy configuration, security event analysis |  |  |
| 12 | Network quality monitoring |  |  |
| 12a | Must provide customizable reports |  |  |
| 12b | Should allow following views:<br>Flexible Web-based view<br>Device views<br>Event logs for the entire setup. |  |  |
| 12c | The management software should support NTA (network traffic analysis) component to display traffic distribution and types on the network. Received, flow packets, analyzes traffic statistics on ports, and display the statistics from various aspects. |  |  |
| 13 | OEM / Standard Software Requirement<br><br>The proposed Network Management Software (NMS) solution shall either:<br><br>a. be developed, owned, and officially supported by the same OEM of the proposed network switches,<br><br>OR<br><br>b. be a commercially available, well-established, industry-standard Network Management Software product widely deployed and supported in enterprise/datacenter network environments.<br><br>The bidder shall not supply any self-developed, customized, locally developed, open-source unsupported, prototype, or ad-hoc software solution as the primary NMS platform unless explicitly approved by PRL.<br><br>The proposed NMS solution must have:<br><br>· proven deployment references in enterprise/campus/datacenter environments,<br>· regular OEM/vendor support and software updates,<br>· official technical documentation,<br>· proper licensing model,<br>· long-term maintainability and upgrade support.<br><br>The bidder must clearly specify:<br><br>· Name of the NMS software,<br>· OEM/Manufacturer details,<br>· Version proposed,<br>· Licensing model,<br>· Support details,<br>· Whether the software is OEM-native or third-party commercial product.<br><br>PRL reserves the right to reject any NMS solution found to be non-standard, insufficiently supported, experimental, or lacking establis **[source text ends here]** |  |  |

## Completeness check

The supplied text contains clauses 1–9, 10a–10i, 11a–11b, 12a–12c, and 13.
The existing gap matrix has an analysis row for each of these references, so
numbered coverage is complete. The matrix is an analysis and is not a verbatim
copy; this file preserves the available source wording. Clause 13's final
sentence is truncated in the supplied text, and no omitted ending has been
inferred.
