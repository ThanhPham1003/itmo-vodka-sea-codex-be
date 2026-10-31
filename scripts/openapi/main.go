// Command openapi generates the HTTP contract from DTOs and the verified route inventory.
// Run only after build and tests pass: go run ./scripts/openapi
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	am "sea-codex-server/internal/audit/models"
	ad "sea-codex-server/internal/auth/dto"
	dd "sea-codex-server/internal/dossier/dto"
	dm "sea-codex-server/internal/dossier/models"
	hd "sea-codex-server/internal/health/dto"
)

type obj = map[string]any

var schemas = obj{}

func ref(name string) obj { return obj{"$ref": "#/components/schemas/" + name} }
func typeName(t reflect.Type) string {
	module := "Common"
	for _, part := range []string{"auth", "audit", "dossier", "health"} {
		if strings.Contains(t.PkgPath(), "/"+part+"/") {
			module = strings.ToUpper(part[:1]) + part[1:]
			break
		}
	}
	return module + strings.ReplaceAll(strings.ReplaceAll(t.Name(), "[", "_"), "]", "")
}
func schema(t reflect.Type) obj {
	if t == nil {
		return obj{"type": "object", "nullable": true}
	}
	if t.Kind() == reflect.Pointer {
		return obj{"allOf": []any{schema(t.Elem())}, "nullable": true}
	}
	if t == reflect.TypeFor[uuid.UUID]() {
		return obj{"type": "string", "format": "uuid"}
	}
	if t == reflect.TypeFor[time.Time]() {
		return obj{"type": "string", "format": "date-time"}
	}
	if t == reflect.TypeFor[json.RawMessage]() {
		return obj{}
	}
	switch t.Kind() {
	case reflect.String:
		return obj{"type": "string"}
	case reflect.Bool:
		return obj{"type": "boolean"}
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint64:
		return obj{"type": "integer", "format": "int64"}
	case reflect.Float32, reflect.Float64:
		return obj{"type": "number", "format": "double"}
	case reflect.Slice, reflect.Array:
		return obj{"type": "array", "items": schema(t.Elem())}
	case reflect.Map:
		return obj{"type": "object", "additionalProperties": schema(t.Elem())}
	case reflect.Struct:
		name := typeName(t)
		if _, ok := schemas[name]; ok {
			return ref(name)
		}
		def := obj{"type": "object"}
		schemas[name] = def
		properties := obj{}
		required := []string{}
		var fields func(reflect.Type)
		fields = func(st reflect.Type) {
			for i := 0; i < st.NumField(); i++ {
				f := st.Field(i)
				tag := f.Tag.Get("json")
				name := strings.Split(tag, ",")[0]
				if name == "-" || !f.IsExported() {
					continue
				}
				if f.Anonymous && name == "" && f.Type.Kind() == reflect.Struct {
					fields(f.Type)
					continue
				}
				if name == "" {
					name = f.Name
				}
				property := schema(f.Type)
				for _, rule := range strings.Split(f.Tag.Get("binding"), ",") {
					if rule == "required" {
						required = append(required, name)
					}
					parts := strings.SplitN(rule, "=", 2)
					if len(parts) != 2 {
						continue
					}
					key, val := parts[0], parts[1]
					if key == "oneof" {
						property["enum"] = strings.Fields(val)
						continue
					}
					number, err := strconv.Atoi(val)
					if err != nil {
						continue
					}
					if key == "min" || key == "gte" {
						switch f.Type.Kind() {
						case reflect.String:
							property["minLength"] = number
						case reflect.Slice:
							property["minItems"] = number
						default:
							property["minimum"] = number
						}
					}
					if key == "max" || key == "lte" {
						switch f.Type.Kind() {
						case reflect.String:
							property["maxLength"] = number
						case reflect.Slice:
							property["maxItems"] = number
						default:
							property["maximum"] = number
						}
					}
				}
				if strings.Contains(f.Tag.Get("binding"), "email") {
					property["format"] = "email"
				}
				if name == "password" {
					property["writeOnly"] = true
					property["format"] = "password"
				}
				properties[name] = property
			}
		}
		fields(t)
		def["properties"] = properties
		if len(required) > 0 {
			def["required"] = required
		}
		if strings.HasSuffix(t.Name(), "Request") {
			def["additionalProperties"] = false
		}
		return ref(name)
	}
	return obj{}
}
func of(v any) obj { return schema(reflect.TypeOf(v)) }
func list(v any) obj {
	return obj{"type": "object", "required": []string{"items", "total", "limit", "offset"}, "properties": obj{"items": obj{"type": "array", "items": of(v)}, "total": obj{"type": "integer"}, "limit": obj{"type": "integer"}, "offset": obj{"type": "integer"}}}
}
func envelope(data obj) obj {
	return obj{"type": "object", "required": []string{"code", "message", "data"}, "properties": obj{"code": obj{"type": "integer"}, "message": obj{"type": "string"}, "data": data}}
}
func param(name, where, desc string, s obj) obj {
	return obj{"name": name, "in": where, "required": where == "path", "description": desc, "schema": s}
}
func query(name, desc string) obj { return param(name, "query", desc, obj{"type": "string"}) }
func page() []any {
	return []any{param("limit", "query", "Page size, default 20", obj{"type": "integer", "minimum": 1, "maximum": 100, "default": 20}), param("offset", "query", "Zero-based offset", obj{"type": "integer", "minimum": 0, "default": 0})}
}

var paths = obj{}
var pathParams = regexp.MustCompile(`\{([^}]+)\}`)
var count int

func add(method, path, id, summary, description, permission string, status int, request, response obj, parameters ...any) {
	path = "/api/v1" + path
	if id == "health" {
		path = "/health"
	}
	params := []any{param("X-Request-ID", "header", "Optional request correlation ID", obj{"type": "string"})}
	for _, match := range pathParams.FindAllStringSubmatch(path, -1) {
		s := obj{"type": "string", "format": "uuid"}
		if match[1] == "code" {
			s = obj{"type": "string"}
		}
		params = append(params, param(match[1], "path", "Resource identifier in the authenticated tenant", s))
	}
	params = append(params, parameters...)
	responses := obj{}
	for _, code := range []int{400, 401, 403, 404, 409, 503} {
		responses[strconv.Itoa(code)] = obj{"description": map[int]string{400: "Invalid input", 401: "Missing, invalid or inactive credentials", 403: "Insufficient permissions", 404: "Resource absent or inaccessible", 409: "Stale version or business conflict", 503: "Database, storage or AI unavailable"}[code], "content": obj{"application/json": obj{"schema": ref("Error")}}}
	}
	responses[strconv.Itoa(status)] = obj{"description": "Success", "headers": obj{"X-Request-ID": obj{"schema": obj{"type": "string"}}}, "content": obj{"application/json": obj{"schema": envelope(response)}}}
	security := []any{obj{"PasetoBearer": []string{}}}
	if permission == "public" {
		security = []any{}
	}
	if id == "signup" || id == "login" {
		responses["429"] = obj{"description": "Rate limit exceeded; Retry-After gives seconds", "content": obj{"application/json": obj{"schema": ref("Error")}}}
	}
	if id == "fileContent" {
		responses["200"] = obj{"description": "Private original or derived file. Audit recorded before streaming. Supports HTTP range requests.", "content": obj{"application/pdf": obj{"schema": obj{"type": "string", "format": "binary"}}, "image/jpeg": obj{"schema": obj{"type": "string", "format": "binary"}}, "image/png": obj{"schema": obj{"type": "string", "format": "binary"}}}}
	}
	op := obj{"operationId": id, "summary": summary, "description": description, "security": security, "x-permission": permission, "parameters": params, "responses": responses}
	if request != nil {
		op["requestBody"] = obj{"required": true, "content": obj{"application/json": obj{"schema": request}}}
	}
	if _, ok := paths[path]; !ok {
		paths[path] = obj{}
	}
	paths[path].(obj)[strings.ToLower(method)] = op
	count++
}
func main() {
	schemas["Error"] = envelope(obj{"type": "object", "nullable": true})
	null := obj{"type": "object", "nullable": true}
	add("GET", "", "health", "Liveness", "Startup requires a reachable database and initialized schema.", "public", 200, nil, of(hd.HealthResponse{}))
	add("POST", "/auth/signup", "signup", "Register company and first administrator", "Company, company_admin and audit are created atomically. Tenant and role cannot be supplied. Password: at least 12 characters, at most 72 UTF-8 bytes.", "public", 201, of(ad.SignupRequest{}), of(ad.UserResponse{}))
	add("POST", "/auth/login", "login", "Log in with username or email", "Returns a PASETO v4.public token. Roles and tenant are reloaded from the DB on every protected request.", "public", 200, of(ad.LoginRequest{}), of(ad.LoginResponse{}))
	add("GET", "/auth/me", "me", "Current account", "Inactive user/company tokens are rejected.", "authenticated", 200, nil, of(ad.UserResponse{}))
	add("GET", "/auth/users", "listUsers", "List company members", "Only company_admin may manage members.", "company_admin", 200, nil, list(ad.UserResponse{}), page()...)
	add("POST", "/auth/users", "createUser", "Create company member", "Roles: broker, legal_specialist, notary, manager, company_admin. Additional permissions are additive. broker cannot receive PII/image/correction/review privileges.", "company_admin", 201, of(ad.CreateUserRequest{}), of(ad.UserResponse{}))
	add("PATCH", "/auth/users/{id}/access", "updateUserAccess", "Update role, permissions and activation", "Replaces additional permission list. Last active company_admin cannot be disabled or demoted. platform_admin cannot be assigned.", "company_admin", 200, of(ad.UpdateAccessRequest{}), of(ad.UserResponse{}))
	for _, c := range []struct {
		path, prefix, label    string
		req, model, definition any
	}{{"document-types", "Document", "document type", dd.DocumentTypeRequest{}, dm.DocumentType{}, dm.DocumentDefinition{}}, {"dossier-types", "DossierType", "dossier type", dd.DossierTypeRequest{}, dm.DossierType{}, dm.DossierDefinition{}}} {
		base := "/" + c.path
		add("GET", base, "list"+c.prefix, "List "+c.label+"s", "Includes inactive entries; tenant scoped.", "dossier:read", 200, nil, list(c.model), page()...)
		add("POST", base, "create"+c.prefix, "Create "+c.label, "Fields are at the top level (name, description, fields/documents/rules), not nested in definition. Leave new field keys, requirement keys, special-check/rule codes empty; server generates them. Document schema_version=0 pins its current version.", "company_admin", 201, of(c.req), of(c.model))
		add("GET", base+"/{code}", "get"+c.prefix, "Get "+c.label, "Returns current version and definition.", "dossier:read", 200, nil, of(c.model))
		add("PUT", base+"/{code}", "update"+c.prefix, "Publish new "+c.label+" version", "expected_version must match current_version. Send the full definition, retaining existing keys/codes; leave new keys/codes empty. Old dossiers retain their pinned snapshots.", "company_admin", 200, obj{"allOf": []any{of(c.req), obj{"type": "object", "required": []string{"expected_version"}}}}, of(c.model))
		add("DELETE", base+"/{code}", "archive"+c.prefix, "Archive "+c.label, "Marks inactive; historical configurations and dossiers are retained. Reactivate via PUT with active=true.", "company_admin", 200, nil, null)
		req := of(dd.SuggestionRequest{})
		if c.path == "dossier-types" {
			req = obj{"type": "object", "additionalProperties": false, "required": []string{"description"}, "properties": obj{"description": obj{"type": "string", "maxLength": 4000}, "document_types": obj{"type": "array", "maxItems": 50, "items": obj{"type": "string"}}}}
		}
		add("POST", base+"/suggestions", "suggest"+c.prefix, "Suggest draft "+c.label, "Sends configuration only to the configured AI provider. Returns an unsaved draft; admin edits and POSTs it explicitly. Never activates AI suggestions.", "company_admin", 200, req, obj{"type": "object", "properties": obj{"draft": obj{"type": "boolean", "enum": []bool{true}}, "definition": of(c.definition)}})
	}
	add("POST", "/dossier-types/{code}/comparison-rules/suggestions", "suggestRules", "Suggest draft comparison rules", "References and operators are validated against the current dossier-type schema. Saving selected rules requires PUT with expected_version. No dossier PII is sent.", "company_admin", 200, of(dd.SuggestionRequest{}), obj{"type": "object", "properties": obj{"draft": obj{"type": "boolean"}, "configuration_version": obj{"type": "integer"}, "rules": obj{"type": "array", "items": of(dm.ComparisonRule{})}}})
	add("POST", "/dossiers", "createDossier", "Create dossier and pinned checklist", "company_id/creator are derived from the current account. Priority 0..5.", "dossier:create", 201, of(dd.CreateDossierRequest{}), of(dm.Dossier{}))
	filters := page()
	for _, name := range []string{"status", "dossier_type", "search", "creator_id", "assignee_id", "from", "to", "timezone"} {
		filters = append(filters, query(name, map[string]string{"from": "Date or RFC3339. Filters status-history transitions when date range is supplied.", "to": "Date inclusive; RFC3339 exclusive.", "timezone": "IANA timezone, default Asia/Ho_Chi_Minh."}[name]))
	}
	add("GET", "/dossiers", "listDossiers", "List accessible dossiers", "Default current-status filtering. With from/to, returns distinct dossiers with a matching status-history transition in the range, regardless of current status. read_all sees the tenant; otherwise own or assigned dossiers only.", "dossier:read", 200, nil, list(dm.Dossier{}), filters...)
	add("GET", "/dossiers/review-queue", "reviewQueue", "Review queue", "VALID_DOCUMENT only; assigned to caller unless company_admin. Order: priority descending then oldest first.", "dossier:review", 200, nil, list(dm.Dossier{}), page()...)
	add("GET", "/dossiers/{id}", "dossierDetail", "Get dossier, checklist, evidence and decisions", "PII values are masked for every role. Raw excerpts and issue evidence are redacted; use explicit reveal. Storage keys, machine values and decision snapshots are never serialized. Poll version/file status to track processing.", "dossier:read", 200, nil, of(dd.Detail{}))
	add("POST", "/dossiers/{id}/files", "uploadFiles", "Upload files in a new batch", "Multipart files: JPEG/PNG/PDF. Optional document_types, roles, replaces_file_ids arrays must match files in count/order. Each file has an independent UPLOADED/DUPLICATE/REJECTED result. Defaults: 10 files/request, 20 MiB/file, 100 PDF pages. Upload hints do not bypass classification. Approved/rejected dossiers are locked. Processing requires the enabled worker.", "dossier:upload", 200, nil, of(dd.UploadResult{}))
	paths["/api/v1/dossiers/{id}/files"].(obj)["post"].(obj)["requestBody"] = obj{"required": true, "content": obj{"multipart/form-data": obj{"schema": obj{"type": "object", "required": []string{"files"}, "properties": obj{"files": obj{"type": "array", "minItems": 1, "maxItems": 10, "items": obj{"type": "string", "format": "binary"}}, "document_types": obj{"type": "array", "items": obj{"type": "string"}}, "roles": obj{"type": "array", "items": obj{"type": "string"}}, "replaces_file_ids": obj{"type": "array", "items": obj{"type": "string"}}}}}}}
	add("GET", "/dossiers/{id}/files/{file_id}/content", "fileContent", "View original or derived file", "Requires read access and image_view; broker is denied. VIEW_FILE audit and no-store response.", "dossier:image_view", 200, nil, null)
	add("POST", "/dossiers/{id}/files/{file_id}/retry", "retryFile", "Retry failed source file", "Only FAILED source files. New claim rejects late results from the previous attempt.", "dossier:upload", 202, of(dd.VersionRequest{}), null)
	add("PATCH", "/dossiers/{id}/files/{file_id}/classification", "classifyFile", "Confirm or correct document classification", "Provide document_group_id for one group in a mixed PDF; omit to apply to the source. Re-extracts asynchronously under the pinned schema.", "dossier:correct", 202, of(dd.ClassificationRequest{}), null)
	add("PATCH", "/dossiers/{id}/assignment", "assignDossier", "Assign responsible member and priority", "Assignee must be active, same-company and have dossier:read; broker cannot be assigned. assignee_id=null unassigns. Review also requires dossier:review. Assignment does not clear a request for changes.", "dossier:assign", 200, of(dd.AssignmentRequest{}), null)
	add("POST", "/dossiers/{id}/requirements", "addRequirement", "Activate conditional requirement", "Document type and role must be in the pinned configuration. min_count defaults to 1. Mandatory requirements cannot be duplicated.", "dossier:upload", 201, of(dd.RequirementRequest{}), null)
	add("DELETE", "/dossiers/{id}/requirements/{requirement_id}", "waiveRequirement", "Mark conditional requirement unnecessary", "Only conditional requirements; reason required. Does not delete historical configuration.", "dossier:upload", 200, of(dd.ReasonRequest{}), null)
	add("PATCH", "/dossiers/{id}/fields/{field_id}", "correctField", "Correct extracted field and reconcile", "Reasons: OCR_ERROR, SOURCE_ERROR, ADMIN_UNIT_CHANGE, OTHER (requires note). Records private before/after. Re-extraction preserves human corrections unless the target group is explicitly reclassified.", "dossier:correct", 200, of(dd.CorrectionRequest{}), null)
	add("POST", "/dossiers/{id}/issues/{issue_id}/acknowledge", "acknowledgeIssue", "Acknowledge warning with reason", "Only open WARNING; cannot acknowledge BLOCKER. Input changes invalidate stale acknowledgements.", "dossier:review", 200, of(dd.ReasonRequest{}), null)
	add("POST", "/dossiers/{id}/decisions", "reviewDossier", "Approve, reject or request changes", "Requires assignment unless company_admin. APPROVE needs VALID_DOCUMENT, no processing/missing documents, no pending/open warning/blocker and valid required fields. REJECT/REQUEST_CHANGES require reason. Stores private decision snapshot atomically with audit/status history.", "dossier:review", 200, of(dd.ReviewRequest{}), of(dm.Decision{}))
	add("POST", "/dossiers/{id}/reopen", "reopenDossier", "Reopen approved dossier", "Reason and current expected_version required. Retains decisions and runs reconciliation; another approval is required.", "dossier:reopen", 200, of(dd.ReasonRequest{}), null)
	add("POST", "/dossiers/{id}/pii/reveal", "revealPII", "Reveal specific fields", "Read access and pii:view required; broker denied. VIEW_PII audit contains field IDs, never values. Response must not be cached.", "pii:view", 200, of(dd.RevealRequest{}), obj{"type": "array", "items": of(dm.ExtractedField{})})
	dashboard := obj{"type": "object", "properties": obj{"metric": obj{"type": "string", "enum": []string{"transitions", "unique", "current"}}, "timezone": obj{"type": "string"}, "from": of(time.Time{}), "to_exclusive": of(time.Time{}), "as_of": of(time.Time{}), "range_applies": obj{"type": "boolean"}, "totals": of(map[string]int{}), "daily": obj{"type": "array", "items": obj{"type": "object", "properties": obj{"date": obj{"type": "string", "format": "date"}, "counts": of(map[string]int{})}}}}}
	add("GET", "/dashboard/statuses", "dashboard", "Status metrics and daily series", "Default transitions counts each entry into a status in the period. unique counts distinct dossier/status, daily counts unique dossier/status/day. current is the current snapshot and returns range_applies=false. Date-only to is inclusive; timestamps are exclusive. Default today in Asia/Ho_Chi_Minh, maximum 366 days. Drill down with GET /dossiers using status/from/to/timezone/type.", "dashboard:read", 200, nil, dashboard, query("from", "Date or RFC3339"), query("to", "Date or RFC3339"), query("timezone", "IANA timezone"), query("metric", "transitions (default), unique, current"), query("dossier_type", "Optional dossier-type code"))
	auditParams := page()
	for _, name := range []string{"action", "resource_id", "actor_id", "dossier_id", "from", "to"} {
		auditParams = append(auditParams, query(name, "Exact filter; from/to are RFC3339 timestamps, to is exclusive"))
	}
	add("GET", "/audit-logs", "auditLogs", "List tenant audit events", "Append-only API; no update/delete endpoints. System actor_id is the zero UUID. Original values and private decision snapshots are excluded.", "audit:read", 200, nil, list(am.AuditLog{}), auditParams...)
	spec := obj{"openapi": "3.0.3", "info": obj{"title": "SEA Codex Backend API", "version": "1.0.0", "description": "Tenant-scoped dossier intake, AI processing and legal review. Bearer tokens are PASETO v4.public. All JSON replies use {code,message,data}; business errors use data:null. Mutation expected_version must match dossier.version from the latest detail, otherwise HTTP 409. Tenant is never selected by client input. Worker disabled by default; configure AI and enable it to process uploads."}, "servers": []any{obj{"url": "http://localhost:8080"}}, "paths": paths, "components": obj{"securitySchemes": obj{"PasetoBearer": obj{"type": "http", "scheme": "bearer", "bearerFormat": "PASETO v4.public"}}, "schemas": schemas}}
	// Domain validation adds requirements beyond transport binding tags.
	for name, fields := range map[string][]string{
		"DossierDocumentDefinition": {"name", "fields"}, "DossierDossierDefinition": {"name", "documents"},
		"DossierDocumentTypeRequest": {"name", "fields"}, "DossierDossierTypeRequest": {"name", "documents"},
		"DossierFieldSchema": {"label", "type"}, "DossierDocumentRequirement": {"document_type"},
		"DossierComparisonRule": {"name", "left", "right", "operator", "severity"}, "DossierFieldReference": {"document_type", "field_key"},
	} {
		if definition, ok := schemas[name].(obj); ok {
			definition["required"] = fields
		}
	}
	encoded, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile("docs/api/api_spec.json", append(encoded, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d operations on %d paths\n", count, len(paths))
}
