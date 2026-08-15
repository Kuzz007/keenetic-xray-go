package main

import "os"

const defaultConfigPath = "/opt/etc/keenetic-xray/config.json"

func configPath() string {
	return envOr("KEENETIC_XRAY_CONFIG", defaultConfigPath)
}

const defaultXrayBinary = "/opt/sbin/xray"

func xrayBinaryPath() string {
	return envOr("KEENETIC_XRAY_BINARY", defaultXrayBinary)
}

const defaultProductionConfigPath = "/opt/var/lib/keenetic-xray/xray-production.json"

func productionConfigPath() string {
	return envOr("KEENETIC_XRAY_PRODUCTION_CONFIG", defaultProductionConfigPath)
}

const defaultPretestConfigPath = "/opt/var/lib/keenetic-xray/xray-pretest.json"

func pretestConfigPath() string {
	return envOr("KEENETIC_XRAY_PRETEST_CONFIG", defaultPretestConfigPath)
}

const defaultOptPath = "/opt"

func optPath() string {
	return envOr("KEENETIC_XRAY_OPT", defaultOptPath)
}

const defaultLogDir = "/opt/var/log/keenetic-xray"

func logDir() string {
	return envOr("KEENETIC_XRAY_LOG_DIR", defaultLogDir)
}

const defaultRunDir = "/opt/var/run"

func runDir() string {
	return envOr("KEENETIC_XRAY_RUN_DIR", defaultRunDir)
}

const defaultAgentTokenFile = "/opt/etc/keenetic-xray/agent-token.secret"

func defaultAgentTokenPath() string {
	return envOr("KEENETIC_XRAY_AGENT_TOKEN_FILE", defaultAgentTokenFile)
}

const defaultLANIPCachePath = "/opt/var/lib/keenetic-xray/router-lan-ip"

func lanIPCachePath() string {
	return envOr("KEENETIC_XRAY_LAN_IP_CACHE", defaultLANIPCachePath)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
