# Include custom targets and environment variables here

E2E_DIR = e2e/playwright
E2E_FAKE_ATLASSIAN_URL ?= http://fake-confluence:8092

# Command-line GO_BUILD_LDFLAGS wins over the Makefile default.
.PHONY: e2e-dist
e2e-dist: ## Builds dist/ with Confluence Cloud OAuth endpoints pointed at the E2E fake. Never ship this bundle.
	$(MAKE) dist GO_BUILD_LDFLAGS='-ldflags="-s -w -X main.atlassianAuthBaseURL=$(E2E_FAKE_ATLASSIAN_URL) -X main.atlassianAPIBaseURL=$(E2E_FAKE_ATLASSIAN_URL) -X main.forgePollIntervalSeconds=1"'

.PHONY: e2e-real
e2e-real: dist $(E2E_DIR)/node_modules ## Runs the real Confluence Cloud journey (needs e2e/playwright/.env).
	cd $(E2E_DIR) && npm run test:real

$(E2E_DIR)/node_modules: $(E2E_DIR)/package-lock.json
	cd $(E2E_DIR) && $(NPM) ci && npx playwright install chromium
	touch $@

.PHONY: e2e-env-up
e2e-env-up: ## Starts the Mattermost stack used by the Playwright suite
	cd $(E2E_DIR) && docker compose up -d --wait

.PHONY: e2e-env-down
e2e-env-down: ## Stops the Playwright Mattermost stack and drops its data
	cd $(E2E_DIR) && docker compose --profile visual down -v

.PHONY: e2e
e2e: e2e-dist $(E2E_DIR)/node_modules ## Runs the Playwright suite against the stack from e2e-env-up
	cd $(E2E_DIR) && npx playwright test
