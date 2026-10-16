# LinaPro Framework - Root Makefile
# ===========================

BACKEND_DIR   := apps/lina-core
FRONTEND_DIR  := apps/lina-vben
TEMP_DIR      := temp
PID_DIR       := $(TEMP_DIR)/pids
BACKEND_PID   := $(PID_DIR)/lina-core.pid
FRONTEND_PID  := $(PID_DIR)/lina-vben.pid
# Allow environment overrides, for example:
#   LINA_CORE_PORT=18080 LINA_VBEN_PORT=3000 LINA_CUSTOMER_PORT=3001 make dev
# 允许环境变量覆盖端口，例如：
#   LINA_CORE_PORT=18080 LINA_VBEN_PORT=3000 LINA_CUSTOMER_PORT=3001 make dev
LINA_CORE_PORT ?= 6030
LINA_VBEN_PORT ?= 6031
LINA_CUSTOMER_PORT ?= 6032
LINA_WEBSITE_PORT ?= 6680
BACKEND_LOG   := $(TEMP_DIR)/lina-core.log
FRONTEND_LOG  := $(TEMP_DIR)/lina-vben.log
EMBED_DIR     := $(BACKEND_DIR)/internal/packed/public
OUTPUT_DIR    := $(TEMP_DIR)/output
LINACTL       := cd hack/tools/linactl && go run .

# Project entrypoints live in the business plugin; host tooling remains unchanged.
env.setup dev build db.init: LINACTL = go run ./apps/lina-plugins/idc-compliance/hack/tools/installation/main.go

# Include split makefile targets.
# 引入拆分后的 Makefile 目标文件。
include hack/makefiles/help.mk
include hack/makefiles/env.mk
include hack/makefiles/dev.mk
include hack/makefiles/build.mk
include hack/makefiles/plugins.mk
include hack/makefiles/image.mk
include hack/makefiles/release.mk
include hack/makefiles/lint.mk
include hack/makefiles/test.mk
include hack/makefiles/i18n.mk
include hack/makefiles/database.mk
include hack/makefiles/agents.mk

# Website renderers use the next two ports after this gateway.
DEV_ARGS += website_port=$(LINA_WEBSITE_PORT)
ifneq ($(origin plan), undefined)
DEV_ARGS += plan=$(plan)
endif
