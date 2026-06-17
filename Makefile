PYTHON ?= python3
PERL ?= perl
CODEX ?= codex
PLUGIN_ROOT ?= plugins
PLUGIN_NAME ?= unui-codex-plugin
PLUGIN_PATH ?= $(PLUGIN_ROOT)/$(PLUGIN_NAME)
MARKETPLACE_ROOT ?= .
MARKETPLACE ?= .agents/plugins/marketplace.json
PLUGIN_MARKETPLACE_CACHE_PATH ?= $${HOME}/.codex/plugins/cache/$(PLUGIN_NAME)
PLUGIN_CACHE_PATH ?= $(PLUGIN_MARKETPLACE_CACHE_PATH)/$(PLUGIN_NAME)
CODEX_GLOBAL_STATE ?= $${HOME}/.codex/.codex-global-state.json
CODEX_WORKSPACE_STATE_CLEANER ?= scripts/clear_codex_workspace_state.py
MCP_CONFIG_FILES ?= $(PLUGIN_PATH)/skills/unui/agents/openai.yaml $(PLUGIN_PATH)/skills/unui-mcp-auth/agents/openai.yaml $(PLUGIN_PATH)/skills/unui-mcp-diagnostics/agents/openai.yaml $(PLUGIN_PATH)/.mcp.json $(PLUGIN_PATH)/scripts/mcp_diagnose_auth.py $(PLUGIN_PATH)/scripts/mcp_diagnose_health.py
MCP_URL_PATTERN ?= https?://(?:localhost:3001|api\.unui\.cc)/v1/mcp
MCP_DEV_URL ?= http://localhost:3001/v1/mcp
MCP_PROD_URL ?= https://api.unui.cc/v1/mcp
PLUGIN_CREATOR_DIR ?= $${HOME}/.codex/skills/.system/plugin-creator
PLUGIN_VALIDATOR ?= $(PLUGIN_CREATOR_DIR)/scripts/validate_plugin.py
PLUGIN_CACHEBUSTER ?= $(PLUGIN_CREATOR_DIR)/scripts/update_plugin_cachebuster.py
MARKETPLACE_NAME_READER ?= $(PLUGIN_CREATOR_DIR)/scripts/read_marketplace_name.py

.DEFAULT_GOAL := validate

.PHONY: help all dev prod validate validate-plugin validate-plugins validate-marketplace install remove refresh-cache check-plugin-validator check-plugin-cachebuster check-marketplace-name-reader check-codex-workspace-state-cleaner

help:
	@printf '%s\n' 'Targets:'
	@printf '%s\n' '  make validate              Run all plugin repo checks.'
	@printf '%s\n' '  make validate-plugins      Validate each plugin against Codex ingestion rules.'
	@printf '%s\n' '  make validate-marketplace  Validate marketplace wiring and plugin identity.'
	@printf '%s\n' '  make install               Install the marketplace and plugin.'
	@printf '%s\n' '  make remove                Remove the plugin, marketplace, and local plugin cache.'
	@printf '%s\n' '  make refresh-cache         Update the plugin cachebuster and reinstall it.'
	@printf '%s\n' '  make dev                   Point plugin MCP configs at the local API.'
	@printf '%s\n' '  make prod                  Point plugin MCP configs at the production API.'
	@printf '%s\n' ''
	@printf '%s\n' 'Variables:'
	@printf '%s\n' '  PLUGIN_ROOT=plugins'
	@printf '%s\n' '  PLUGIN_NAME=unui-codex-plugin'
	@printf '%s\n' '  PLUGIN_PATH=plugins/unui-codex-plugin'
	@printf '%s\n' '  PLUGIN_MARKETPLACE_CACHE_PATH=$$HOME/.codex/plugins/cache/unui-codex-plugin'
	@printf '%s\n' '  PLUGIN_CACHE_PATH=$$HOME/.codex/plugins/cache/unui-codex-plugin/unui-codex-plugin'
	@printf '%s\n' '  CODEX_GLOBAL_STATE=$$HOME/.codex/.codex-global-state.json'
	@printf '%s\n' '  MARKETPLACE_ROOT=.'
	@printf '%s\n' '  MARKETPLACE=.agents/plugins/marketplace.json'
	@printf '%s\n' '  MCP_DEV_URL=http://localhost:3001/v1/mcp'
	@printf '%s\n' '  MCP_PROD_URL=https://api.unui.cc/v1/mcp'
	@printf '%s\n' '  MCP_CONFIG_FILES=plugins/unui-codex-plugin/skills/unui/agents/openai.yaml plugins/unui-codex-plugin/skills/unui-mcp-auth/agents/openai.yaml plugins/unui-codex-plugin/skills/unui-mcp-diagnostics/agents/openai.yaml plugins/unui-codex-plugin/.mcp.json plugins/unui-codex-plugin/scripts/mcp_diagnose_auth.py plugins/unui-codex-plugin/scripts/mcp_diagnose_health.py'
	@printf '%s\n' '  MCP_URL_PATTERN=https?://(?:localhost:3001|api\.unui\.cc)/v1/mcp'
	@printf '%s\n' '  PLUGIN_VALIDATOR=$$HOME/.codex/skills/.system/plugin-creator/scripts/validate_plugin.py'
	@printf '%s\n' '  PLUGIN_CACHEBUSTER=$$HOME/.codex/skills/.system/plugin-creator/scripts/update_plugin_cachebuster.py'

all: validate

dev:
	@$(PERL) -0pi -e 's#$(MCP_URL_PATTERN)#$(MCP_DEV_URL)#g' $(MCP_CONFIG_FILES)
	@printf 'Set plugin MCP configs to %s\n' '$(MCP_DEV_URL)'

prod:
	@$(PERL) -0pi -e 's#$(MCP_URL_PATTERN)#$(MCP_PROD_URL)#g' $(MCP_CONFIG_FILES)
	@printf 'Set plugin MCP configs to %s\n' '$(MCP_PROD_URL)'

validate: validate-plugins validate-marketplace
	@printf '%s\n' 'All plugin checks passed.'

validate-plugin: validate-plugins

validate-plugins: check-plugin-validator
	@set -eu; \
	if [ ! -d "$(PLUGIN_ROOT)" ]; then \
		printf '%s\n' 'Missing plugin root: $(PLUGIN_ROOT)' >&2; \
		exit 1; \
	fi; \
	plugin_paths=$$(find "$(PLUGIN_ROOT)" -mindepth 1 -maxdepth 1 -type d | sort); \
	if [ -z "$$plugin_paths" ]; then \
		printf '%s\n' 'No plugin directories found under $(PLUGIN_ROOT)' >&2; \
		exit 1; \
	fi; \
	for plugin_path in $$plugin_paths; do \
		printf 'Validating %s with Codex plugin validator\n' "$$plugin_path"; \
		validator_output=$$("$(PYTHON)" "$(PLUGIN_VALIDATOR)" "$$plugin_path" 2>&1) || { \
			printf '%s\n' "$$validator_output" >&2; \
			exit 1; \
		}; \
		printf 'Plugin validation passed: %s\n' "$$plugin_path"; \
	done

validate-marketplace:
	@$(PYTHON) scripts/validate_plugin_repo.py --marketplace "$(MARKETPLACE)"

install: validate check-marketplace-name-reader
	@set -eu; \
	marketplace_name=$$("$(PYTHON)" "$(MARKETPLACE_NAME_READER)" --marketplace-path "$(MARKETPLACE)"); \
	printf '%s\n' 'Refreshing marketplace registration.'; \
	"$(CODEX)" plugin marketplace add "$(MARKETPLACE_ROOT)" >/dev/null; \
	printf '%s\n' 'Installing plugin.'; \
	"$(CODEX)" plugin add "$(PLUGIN_NAME)@$$marketplace_name" >/dev/null; \
	printf 'Added plugin %s from marketplace %s.\n' "$(PLUGIN_NAME)" "$$marketplace_name"; \
	printf '%s\n' 'Plugin installed. Start a new Codex thread to use the updated plugin.'

remove: check-marketplace-name-reader check-codex-workspace-state-cleaner
	@set -u; \
	marketplace_name=$$("$(PYTHON)" "$(MARKETPLACE_NAME_READER)" --marketplace-path "$(MARKETPLACE)" 2>/dev/null || printf '%s' "$(PLUGIN_NAME)"); \
	printf 'Removing plugin %s from marketplace %s.\n' "$(PLUGIN_NAME)" "$$marketplace_name"; \
	"$(CODEX)" plugin remove "$(PLUGIN_NAME)@$$marketplace_name" >/dev/null 2>&1 || true; \
	printf 'Removing marketplace %s.\n' "$$marketplace_name"; \
	"$(CODEX)" plugin marketplace remove "$$marketplace_name" >/dev/null 2>&1 || true; \
	printf 'Removing plugin cache %s and %s.\n' "$(PLUGIN_CACHE_PATH)" "$(PLUGIN_MARKETPLACE_CACHE_PATH)"; \
	rm -rf "$(PLUGIN_CACHE_PATH)" "$(PLUGIN_MARKETPLACE_CACHE_PATH)"; \
	printf 'Removing Codex Desktop workspace state for %s.\n' "$(abspath $(MARKETPLACE_ROOT))"; \
	"$(PYTHON)" "$(CODEX_WORKSPACE_STATE_CLEANER)" \
		--state "$(CODEX_GLOBAL_STATE)" \
		--path "$(abspath $(MARKETPLACE_ROOT))" \
		--path "$(abspath $(PLUGIN_PATH))"; \
	printf '%s\n' 'Plugin, marketplace, local plugin cache, and saved workspace state removed.'; \
	printf '%s\n' 'If Codex Desktop is open, quit it before running remove so saved workspace state is not written back on restart.'

refresh-cache: validate check-plugin-cachebuster check-marketplace-name-reader
	@set -eu; \
	marketplace_name=$$("$(PYTHON)" "$(MARKETPLACE_NAME_READER)" --marketplace-path "$(MARKETPLACE)"); \
	printf '%s\n' 'Updating plugin cachebuster.'; \
	"$(PYTHON)" "$(PLUGIN_CACHEBUSTER)" "$(PLUGIN_PATH)"; \
	printf '%s\n' 'Refreshing marketplace registration.'; \
	"$(CODEX)" plugin marketplace add "$(MARKETPLACE_ROOT)" >/dev/null; \
	printf '%s\n' 'Installing refreshed plugin.'; \
	"$(CODEX)" plugin add "$(PLUGIN_NAME)@$$marketplace_name" >/dev/null; \
	printf 'Added plugin %s from marketplace %s.\n' "$(PLUGIN_NAME)" "$$marketplace_name"; \
	printf '%s\n' 'Plugin cache refreshed. Start a new Codex thread to use the updated plugin.'

check-plugin-validator:
	@if [ ! -f "$(PLUGIN_VALIDATOR)" ]; then \
		printf '%s\n' 'Missing Codex plugin validator helper.' >&2; \
		printf '%s\n' 'Set PLUGIN_VALIDATOR=/path/to/validate_plugin.py or install the plugin-creator skill.' >&2; \
		exit 1; \
	fi

check-plugin-cachebuster:
	@if [ ! -f "$(PLUGIN_CACHEBUSTER)" ]; then \
		printf '%s\n' 'Missing Codex plugin cachebuster helper.' >&2; \
		printf '%s\n' 'Set PLUGIN_CACHEBUSTER=/path/to/update_plugin_cachebuster.py or install the plugin-creator skill.' >&2; \
		exit 1; \
	fi

check-marketplace-name-reader:
	@if [ ! -f "$(MARKETPLACE_NAME_READER)" ]; then \
		printf '%s\n' 'Missing Codex marketplace name reader.' >&2; \
		printf '%s\n' 'Set MARKETPLACE_NAME_READER=/path/to/read_marketplace_name.py or install the plugin-creator skill.' >&2; \
		exit 1; \
	fi

check-codex-workspace-state-cleaner:
	@if [ ! -f "$(CODEX_WORKSPACE_STATE_CLEANER)" ]; then \
		printf '%s\n' 'Missing Codex workspace state cleaner.' >&2; \
		printf '%s\n' 'Set CODEX_WORKSPACE_STATE_CLEANER=/path/to/clear_codex_workspace_state.py.' >&2; \
		exit 1; \
	fi
