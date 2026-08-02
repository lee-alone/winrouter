export namespace autostart {
	
	export class Status {
	    enabled: boolean;
	    registered_command?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.registered_command = source["registered_command"];
	    }
	}

}

export namespace config {
	
	export class MVPProxy {
	    type: string;
	    server: string;
	    port: number;
	    method?: string;
	    username?: string;
	    password?: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPProxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.method = source["method"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class MVPDNSServer {
	    type: string;
	    server: string;
	    port: number;
	    server_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPDNSServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.server_name = source["server_name"];
	    }
	}
	export class MVPDNS {
	    domestic: MVPDNSServer;
	    global: MVPDNSServer;
	
	    static createFrom(source: any = {}) {
	        return new MVPDNS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domestic = this.convertValues(source["domestic"], MVPDNSServer);
	        this.global = this.convertValues(source["global"], MVPDNSServer);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MVPDomestic {
	    cidrs: string[];
	    domain_suffixes: string[];
	
	    static createFrom(source: any = {}) {
	        return new MVPDomestic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cidrs = source["cidrs"];
	        this.domain_suffixes = source["domain_suffixes"];
	    }
	}
	export class MVPRuleSet {
	    tag: string;
	    kind: string;
	    action: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPRuleSet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag = source["tag"];
	        this.kind = source["kind"];
	        this.action = source["action"];
	        this.path = source["path"];
	    }
	}
	export class MVPCustomRule {
	    name: string;
	    type: string;
	    value: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPCustomRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.value = source["value"];
	        this.action = source["action"];
	    }
	}
	export class MVPDirectPrefix {
	    prefix: string;
	    bind_interface: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPDirectPrefix(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.bind_interface = source["bind_interface"];
	    }
	}
	export class MVPInterface {
	    guid: string;
	    bind_interface: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.bind_interface = source["bind_interface"];
	    }
	}
	export class MVPTUN {
	    prefix: string;
	    stack: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPTUN(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.stack = source["stack"];
	    }
	}
	export class MVPConfig {
	    schema_version: number;
	    mode: string;
	    tun: MVPTUN;
	    interface_a: MVPInterface;
	    interface_b: MVPInterface;
	    direct_prefixes: MVPDirectPrefix[];
	    custom_rules?: MVPCustomRule[];
	    rule_sets?: MVPRuleSet[];
	    domestic: MVPDomestic;
	    dns: MVPDNS;
	    proxy?: MVPProxy;
	    ipv6: string;
	
	    static createFrom(source: any = {}) {
	        return new MVPConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.mode = source["mode"];
	        this.tun = this.convertValues(source["tun"], MVPTUN);
	        this.interface_a = this.convertValues(source["interface_a"], MVPInterface);
	        this.interface_b = this.convertValues(source["interface_b"], MVPInterface);
	        this.direct_prefixes = this.convertValues(source["direct_prefixes"], MVPDirectPrefix);
	        this.custom_rules = this.convertValues(source["custom_rules"], MVPCustomRule);
	        this.rule_sets = this.convertValues(source["rule_sets"], MVPRuleSet);
	        this.domestic = this.convertValues(source["domestic"], MVPDomestic);
	        this.dns = this.convertValues(source["dns"], MVPDNS);
	        this.proxy = this.convertValues(source["proxy"], MVPProxy);
	        this.ipv6 = source["ipv6"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	
	
	
	
	export class RulePreview {
	    position: number;
	    category: string;
	    match: string[];
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new RulePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.category = source["category"];
	        this.match = source["match"];
	        this.action = source["action"];
	    }
	}

}

export namespace core {
	
	export class Status {
	    state: string;
	    pid?: number;
	    generation: number;
	    restart_attempts: number;
	    abnormal: boolean;
	    last_error?: string;
	    core_log?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.pid = source["pid"];
	        this.generation = source["generation"];
	        this.restart_attempts = source["restart_attempts"];
	        this.abnormal = source["abnormal"];
	        this.last_error = source["last_error"];
	        this.core_log = source["core_log"];
	    }
	}

}

export namespace dnssettings {
	
	export class Preset {
	    id: string;
	    name: string;
	    scope: string;
	    type: string;
	    server: string;
	    port: number;
	    server_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scope = source["scope"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.server_name = source["server_name"];
	    }
	}
	export class Server {
	    preset_id?: string;
	    type: string;
	    server: string;
	    port: number;
	    server_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preset_id = source["preset_id"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.server_name = source["server_name"];
	    }
	}
	export class Settings {
	    schema_version: number;
	    domestic: Server;
	    global: Server;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema_version = source["schema_version"];
	        this.domestic = this.convertValues(source["domestic"], Server);
	        this.global = this.convertValues(source["global"], Server);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace interfacemanager {
	
	export class Candidate {
	    adapter: interfaces.Adapter;
	    eligible: boolean;
	    recommendation_score: number;
	    reasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new Candidate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapter = this.convertValues(source["adapter"], interfaces.Adapter);
	        this.eligible = source["eligible"];
	        this.recommendation_score = source["recommendation_score"];
	        this.reasons = source["reasons"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Diagnostic {
	    code: string;
	    severity: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.severity = source["severity"];
	        this.message = source["message"];
	    }
	}
	export class ResolvedSelection {
	    role: string;
	    saved: interfaces.Identity;
	    match?: interfaces.Match;
	    status: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResolvedSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.saved = this.convertValues(source["saved"], interfaces.Identity);
	        this.match = this.convertValues(source["match"], interfaces.Match);
	        this.status = source["status"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Snapshot {
	    sequence: number;
	    candidates: Candidate[];
	    adapters: interfaces.Adapter[];
	    routes: routes.Route[];
	    topology: interfaces.Topology;
	    interface_a: ResolvedSelection;
	    interface_b: ResolvedSelection;
	    tun?: tunprefix.Allocation;
	    diagnostics: Diagnostic[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sequence = source["sequence"];
	        this.candidates = this.convertValues(source["candidates"], Candidate);
	        this.adapters = this.convertValues(source["adapters"], interfaces.Adapter);
	        this.routes = this.convertValues(source["routes"], routes.Route);
	        this.topology = this.convertValues(source["topology"], interfaces.Topology);
	        this.interface_a = this.convertValues(source["interface_a"], ResolvedSelection);
	        this.interface_b = this.convertValues(source["interface_b"], ResolvedSelection);
	        this.tun = this.convertValues(source["tun"], tunprefix.Allocation);
	        this.diagnostics = this.convertValues(source["diagnostics"], Diagnostic);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace interfaces {
	
	export class Address {
	    ip: string;
	    prefix_length: number;
	
	    static createFrom(source: any = {}) {
	        return new Address(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.prefix_length = source["prefix_length"];
	    }
	}
	export class Adapter {
	    guid: string;
	    luid: number;
	    index: number;
	    ipv6_index: number;
	    mac?: string;
	    friendly_name: string;
	    description: string;
	    status: string;
	    kind: string;
	    candidate: boolean;
	    mtu: number;
	    ipv4_metric: number;
	    ipv6_metric: number;
	    addresses: Address[];
	    gateways: string[];
	    dns_servers: string[];
	
	    static createFrom(source: any = {}) {
	        return new Adapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.luid = source["luid"];
	        this.index = source["index"];
	        this.ipv6_index = source["ipv6_index"];
	        this.mac = source["mac"];
	        this.friendly_name = source["friendly_name"];
	        this.description = source["description"];
	        this.status = source["status"];
	        this.kind = source["kind"];
	        this.candidate = source["candidate"];
	        this.mtu = source["mtu"];
	        this.ipv4_metric = source["ipv4_metric"];
	        this.ipv6_metric = source["ipv6_metric"];
	        this.addresses = this.convertValues(source["addresses"], Address);
	        this.gateways = source["gateways"];
	        this.dns_servers = source["dns_servers"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DirectPrefix {
	    prefix: string;
	    adapter_guid: string;
	    adapter_name: string;
	    adapter_kind: string;
	    action: string;
	    route_metric: number;
	    adapter_state: string;
	
	    static createFrom(source: any = {}) {
	        return new DirectPrefix(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.adapter_guid = source["adapter_guid"];
	        this.adapter_name = source["adapter_name"];
	        this.adapter_kind = source["adapter_kind"];
	        this.action = source["action"];
	        this.route_metric = source["route_metric"];
	        this.adapter_state = source["adapter_state"];
	    }
	}
	export class Identity {
	    guid: string;
	    mac?: string;
	    ipv4_prefixes?: string[];
	    friendly_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Identity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.mac = source["mac"];
	        this.ipv4_prefixes = source["ipv4_prefixes"];
	        this.friendly_name = source["friendly_name"];
	    }
	}
	export class Match {
	    adapter: Adapter;
	    method: string;
	    score: number;
	    reasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new Match(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.adapter = this.convertValues(source["adapter"], Adapter);
	        this.method = source["method"];
	        this.score = source["score"];
	        this.reasons = source["reasons"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PrefixOverlap {
	    first: DirectPrefix;
	    second: DirectPrefix;
	    kind: string;
	    requires_selection: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrefixOverlap(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.first = this.convertValues(source["first"], DirectPrefix);
	        this.second = this.convertValues(source["second"], DirectPrefix);
	        this.kind = source["kind"];
	        this.requires_selection = source["requires_selection"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Topology {
	    prefixes: DirectPrefix[];
	    overlaps: PrefixOverlap[];
	
	    static createFrom(source: any = {}) {
	        return new Topology(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefixes = this.convertValues(source["prefixes"], DirectPrefix);
	        this.overlaps = this.convertValues(source["overlaps"], PrefixOverlap);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace main {
	
	export class ApplicationStatus {
	    name: string;
	    version: string;
	    commit: string;
	    mode: string;
	    coreVersion: string;
	    ready: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ApplicationStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.commit = source["commit"];
	        this.mode = source["mode"];
	        this.coreVersion = source["coreVersion"];
	        this.ready = source["ready"];
	    }
	}
	export class ObservationSnapshot {
	    logs: observability.LogEntry[];
	    probes: observability.ProbeResult[];
	    counters: observability.InterfaceCounter[];
	    rule_sets: observability.RuleSetMetadata[];
	    connections: observability.ConnectionSummary;
	    rule_hits: observability.RuleHit[];
	    traffic_budget: trafficbudget.Status;
	
	    static createFrom(source: any = {}) {
	        return new ObservationSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.logs = this.convertValues(source["logs"], observability.LogEntry);
	        this.probes = this.convertValues(source["probes"], observability.ProbeResult);
	        this.counters = this.convertValues(source["counters"], observability.InterfaceCounter);
	        this.rule_sets = this.convertValues(source["rule_sets"], observability.RuleSetMetadata);
	        this.connections = this.convertValues(source["connections"], observability.ConnectionSummary);
	        this.rule_hits = this.convertValues(source["rule_hits"], observability.RuleHit);
	        this.traffic_budget = this.convertValues(source["traffic_budget"], trafficbudget.Status);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace nodes {
	
	export class Input {
	    id?: string;
	    name: string;
	    type: string;
	    server: string;
	    port: number;
	    username?: string;
	    password?: string;
	    clear_password?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.clear_password = source["clear_password"];
	    }
	}
	export class Node {
	    id: string;
	    name: string;
	    type: string;
	    server: string;
	    resolved_ip?: string;
	    port: number;
	    username?: string;
	    has_password: boolean;
	    selected: boolean;
	    subscription_id?: string;
	    favorite: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Node(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.server = source["server"];
	        this.resolved_ip = source["resolved_ip"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.has_password = source["has_password"];
	        this.selected = source["selected"];
	        this.subscription_id = source["subscription_id"];
	        this.favorite = source["favorite"];
	    }
	}
	export class TestResult {
	    node_id: string;
	    available: boolean;
	    latency_ms: number;
	    status?: number;
	    error?: string;
	    // Go type: time
	    tested_at: any;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.node_id = source["node_id"];
	        this.available = source["available"];
	        this.latency_ms = source["latency_ms"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.tested_at = this.convertValues(source["tested_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace observability {
	
	export class BundlePreview {
	    files: string[];
	    log_count: number;
	    probe_count: number;
	    rule_set_count: number;
	    sensitive_notes: string[];
	
	    static createFrom(source: any = {}) {
	        return new BundlePreview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.log_count = source["log_count"];
	        this.probe_count = source["probe_count"];
	        this.rule_set_count = source["rule_set_count"];
	        this.sensitive_notes = source["sensitive_notes"];
	    }
	}
	export class ConnectionSummary {
	    active_tcp: number;
	    established_tcp: number;
	    listening_tcp: number;
	    udp_endpoints: number;
	    // Go type: time
	    sampled_at: any;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active_tcp = source["active_tcp"];
	        this.established_tcp = source["established_tcp"];
	        this.listening_tcp = source["listening_tcp"];
	        this.udp_endpoints = source["udp_endpoints"];
	        this.sampled_at = this.convertValues(source["sampled_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InterfaceCounter {
	    guid: string;
	    name: string;
	    received_bytes: number;
	    transmitted_bytes: number;
	    // Go type: time
	    sampled_at: any;
	
	    static createFrom(source: any = {}) {
	        return new InterfaceCounter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.name = source["name"];
	        this.received_bytes = source["received_bytes"];
	        this.transmitted_bytes = source["transmitted_bytes"];
	        this.sampled_at = this.convertValues(source["sampled_at"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class LogEntry {
	    // Go type: time
	    time: any;
	    level: string;
	    component: string;
	    message: string;
	    correlation_id?: string;
	    fields?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.level = source["level"];
	        this.component = source["component"];
	        this.message = source["message"];
	        this.correlation_id = source["correlation_id"];
	        this.fields = source["fields"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProbeRequest {
	    protocol: string;
	    target: string;
	    dns_name?: string;
	    expected_interface?: string;
	    timeout_ms?: number;
	
	    static createFrom(source: any = {}) {
	        return new ProbeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.target = source["target"];
	        this.dns_name = source["dns_name"];
	        this.expected_interface = source["expected_interface"];
	        this.timeout_ms = source["timeout_ms"];
	    }
	}
	export class ProbeResult {
	    // Go type: time
	    time: any;
	    protocol: string;
	    target: string;
	    source_address?: string;
	    expected_interface?: string;
	    actual_interface?: string;
	    duration_ms: number;
	    success: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = this.convertValues(source["time"], null);
	        this.protocol = source["protocol"];
	        this.target = source["target"];
	        this.source_address = source["source_address"];
	        this.expected_interface = source["expected_interface"];
	        this.actual_interface = source["actual_interface"];
	        this.duration_ms = source["duration_ms"];
	        this.success = source["success"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RuleHit {
	    outbound: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new RuleHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outbound = source["outbound"];
	        this.count = source["count"];
	    }
	}
	export class RuleSetMetadata {
	    name: string;
	    version: string;
	    sha256: string;
	    source: string;
	    rule_count: number;
	    size: number;
	    load_result: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleSetMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.sha256 = source["sha256"];
	        this.source = source["source"];
	        this.rule_count = source["rule_count"];
	        this.size = source["size"];
	        this.load_result = source["load_result"];
	    }
	}

}

export namespace processrules {
	
	export class Status {
	    type: string;
	    value: string;
	    state: string;
	    matches: number;
	    paths?: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.value = source["value"];
	        this.state = source["state"];
	        this.matches = source["matches"];
	        this.paths = source["paths"];
	        this.message = source["message"];
	    }
	}

}

export namespace recovery {
	
	export class Status {
	    state: string;
	    desired_running: boolean;
	    attempts: number;
	    last_error?: string;
	    last_change?: string;
	    snapshot_sequence?: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.desired_running = source["desired_running"];
	        this.attempts = source["attempts"];
	        this.last_error = source["last_error"];
	        this.last_change = source["last_change"];
	        this.snapshot_sequence = source["snapshot_sequence"];
	    }
	}

}

export namespace routes {
	
	export class Route {
	    prefix: string;
	    interface_luid: number;
	    interface_index: number;
	    metric: number;
	    protocol: number;
	
	    static createFrom(source: any = {}) {
	        return new Route(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.interface_luid = source["interface_luid"];
	        this.interface_index = source["interface_index"];
	        this.metric = source["metric"];
	        this.protocol = source["protocol"];
	    }
	}

}

export namespace rulesets {
	
	export class Source {
	    name: string;
	    url: string;
	    expected_sha256: string;
	    applied_sha256?: string;
	    version?: string;
	    rule_count: number;
	    size: number;
	    // Go type: time
	    updated_at?: any;
	    last_error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.url = source["url"];
	        this.expected_sha256 = source["expected_sha256"];
	        this.applied_sha256 = source["applied_sha256"];
	        this.version = source["version"];
	        this.rule_count = source["rule_count"];
	        this.size = source["size"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.last_error = source["last_error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace srssets {
	
	export class Preset {
	    id: string;
	    name: string;
	    kind: string;
	    url: string;
	    upstream: string;
	    license: string;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.url = source["url"];
	        this.upstream = source["upstream"];
	        this.license = source["license"];
	    }
	}
	export class Source {
	    id: string;
	    name: string;
	    kind: string;
	    preset_id?: string;
	    url: string;
	    expected_sha256?: string;
	    applied_sha256?: string;
	    size: number;
	    // Go type: time
	    updated_at?: any;
	    last_error?: string;
	    enabled: boolean;
	    action: string;
	    upstream?: string;
	    license?: string;
	
	    static createFrom(source: any = {}) {
	        return new Source(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.preset_id = source["preset_id"];
	        this.url = source["url"];
	        this.expected_sha256 = source["expected_sha256"];
	        this.applied_sha256 = source["applied_sha256"];
	        this.size = source["size"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.last_error = source["last_error"];
	        this.enabled = source["enabled"];
	        this.action = source["action"];
	        this.upstream = source["upstream"];
	        this.license = source["license"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace subscriptions {
	
	export class Input {
	    id?: string;
	    name: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.url = source["url"];
	    }
	}
	export class Subscription {
	    id: string;
	    name: string;
	    host: string;
	    node_count: number;
	    // Go type: time
	    updated_at?: any;
	    last_error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Subscription(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.node_count = source["node_count"];
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.last_error = source["last_error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace trafficbudget {
	
	export class Settings {
	    enabled: boolean;
	    budget_gb: number;
	    warning_percent: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.budget_gb = source["budget_gb"];
	        this.warning_percent = source["warning_percent"];
	    }
	}
	export class Status {
	    enabled: boolean;
	    budget_gb: number;
	    warning_percent: number;
	    period: string;
	    used_bytes: number;
	    budget_bytes: number;
	    used_percent: number;
	    warning_reached: boolean;
	    limit_reached: boolean;
	    interface_guid?: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.budget_gb = source["budget_gb"];
	        this.warning_percent = source["warning_percent"];
	        this.period = source["period"];
	        this.used_bytes = source["used_bytes"];
	        this.budget_bytes = source["budget_bytes"];
	        this.used_percent = source["used_percent"];
	        this.warning_reached = source["warning_reached"];
	        this.limit_reached = source["limit_reached"];
	        this.interface_guid = source["interface_guid"];
	    }
	}

}

export namespace tunprefix {
	
	export class Conflict {
	    candidate: string;
	    existing_prefix: string;
	    source: string;
	    interface_index?: number;
	    interface_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Conflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.candidate = source["candidate"];
	        this.existing_prefix = source["existing_prefix"];
	        this.source = source["source"];
	        this.interface_index = source["interface_index"];
	        this.interface_name = source["interface_name"];
	    }
	}
	export class Allocation {
	    prefix: string;
	    reused: boolean;
	    conflicts: Conflict[];
	
	    static createFrom(source: any = {}) {
	        return new Allocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	        this.reused = source["reused"];
	        this.conflicts = this.convertValues(source["conflicts"], Conflict);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

