# org-core

`org-core` is the official LinaPro source plugin for organization management.

English | [简体中文](README.zh-CN.md)

## Scope

This plugin owns:

- department management
- post management
- the optional organization capability consumed by host user management

## Host Boundary

The host keeps user management, authentication, and menu governance. This plugin adds organization-specific APIs, menus, and storage tables under `manifest/sql/`.

## Permission Management Help

Department and position menus are declared under the host `iam` directory alongside users and roles. Startup menu reconciliation updates the existing menu records without changing their permission codes or role grants. The host already hides empty directories, so the former organization directory disappears from navigation.

The published `layout.header.actions.before` slot mounts a floating question-mark help button at the lower right of the page to user, role, menu, department, and position management. The help drawer explains setup steps and permission boundaries in English and Chinese. This integration does not modify host pages or routes.

## Directory Layout

```text
org-core/
  plugin.yaml
  backend/
  frontend/
  manifest/
```

