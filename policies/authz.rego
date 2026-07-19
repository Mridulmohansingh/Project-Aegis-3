# AEGIS Authorization Policy — Open Policy Agent (OPA)
#
# This Rego policy implements Attribute-Based Access Control (ABAC)
# for the AEGIS platform. It enforces:
#   - Role-based permissions
#   - Multi-tenant isolation (organization scope)
#   - Separation of duties for item lifecycle
#   - Resource-level access controls
#
# Evaluated by OPA server at http://localhost:8181/v1/data/aegis/authz

package aegis.authz

import rego.v1

# Default deny
default allow := false

# ──────────────────────────────────────────────
#  Role Permissions
# ──────────────────────────────────────────────

role_permissions := {
    "SUPER_ADMIN":     {"*"},
    "ORG_ADMIN":       {"org:*"},
    "ITEM_AUTHOR":     {"item:create", "item:read_own", "item:update_own", "item:submit_review"},
    "REVIEWER":        {"item:read_assigned", "item:review"},
    "PSYCHOMETRICIAN": {"item:read_assigned", "item:calibrate", "item:read_stats"},
    "APPROVER":        {"item:read", "item:approve"},
    "EXAM_ADMIN":      {"exam:create", "exam:read", "exam:update", "exam:delete",
                        "blueprint:create", "blueprint:read", "blueprint:update",
                        "paper:generate", "center:manage"},
    "SCORER":          {"score:process", "score:verify", "result:read"},
    "PROCTOR":         {"exam:monitor", "incident:create", "incident:read"},
    "AUDITOR":         {"audit:read"},
}

# ──────────────────────────────────────────────
#  Permission Matching
# ──────────────────────────────────────────────

# Check if a permission matches a required permission
# Supports wildcards: "item:*" matches "item:create", "item:read", etc.
# Also supports: "*" matches everything
permission_matches(granted, required) if {
    granted == "*"
}

permission_matches(granted, required) if {
    granted == required
}

permission_matches(granted, required) if {
    endswith(granted, ":*")
    prefix := trim_suffix(granted, "*")
    startswith(required, prefix)
}

# ──────────────────────────────────────────────
#  Main Authorization Rule
# ──────────────────────────────────────────────

# Allow if the user has a role that grants the required permission
# AND the user belongs to the same organization as the resource
allow if {
    some role in input.user.roles
    some perm in role_permissions[role]
    permission_matches(perm, input.required_permission)
    organization_check
}

# Organization isolation: user's org must match the resource's org
organization_check if {
    input.user.organization_id == input.resource.organization_id
}

# Super admins bypass org check
organization_check if {
    some role in input.user.roles
    role == "SUPER_ADMIN"
}

# ──────────────────────────────────────────────
#  Separation of Duties
# ──────────────────────────────────────────────

# Deny if separation of duties is violated
deny_reasons contains "Reviewer cannot be the item author" if {
    input.required_permission == "item:review"
    input.user.id == input.resource.author_id
}

deny_reasons contains "Psychometrician cannot be the author or reviewer" if {
    input.required_permission == "item:calibrate"
    input.user.id == input.resource.author_id
}

deny_reasons contains "Psychometrician cannot be the author or reviewer" if {
    input.required_permission == "item:calibrate"
    input.user.id == input.resource.reviewer_id
}

deny_reasons contains "Approver cannot be the author, reviewer, or psychometrician" if {
    input.required_permission == "item:approve"
    input.user.id == input.resource.author_id
}

deny_reasons contains "Approver cannot be the author, reviewer, or psychometrician" if {
    input.required_permission == "item:approve"
    input.user.id == input.resource.reviewer_id
}

deny_reasons contains "Approver cannot be the author, reviewer, or psychometrician" if {
    input.required_permission == "item:approve"
    input.user.id == input.resource.psychometrician_id
}

# Final decision: allow only if no deny reasons exist
default final_allow := false

final_allow if {
    allow
    count(deny_reasons) == 0
}

# ──────────────────────────────────────────────
#  Owner Access
# ──────────────────────────────────────────────

# Item authors can read/update their own items
allow if {
    input.required_permission == "item:read_own"
    input.user.id == input.resource.author_id
    organization_check
}

allow if {
    input.required_permission == "item:update_own"
    input.user.id == input.resource.author_id
    input.resource.status == "DRAFT"
    organization_check
}

# ──────────────────────────────────────────────
#  Rate Limiting Policy
# ──────────────────────────────────────────────

# Maximum operations per minute per user per resource type
rate_limits := {
    "item:create":   30,
    "paper:generate": 5,
    "exam:create":   10,
    "score:process":  2,
}

default rate_limit := 60

rate_limit_for_action := limit if {
    limit := rate_limits[input.required_permission]
}

rate_limit_for_action := rate_limit if {
    not rate_limits[input.required_permission]
}
