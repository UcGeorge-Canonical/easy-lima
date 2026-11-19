CREATE TABLE registry (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), registry_id TEXT NOT NULL UNIQUE CHECK(length(registry_id)=32),
 schema_version INTEGER NOT NULL CHECK(schema_version=1), generation INTEGER NOT NULL DEFAULT 0,
 recovery_generation INTEGER NOT NULL DEFAULT 0, recovery_digest TEXT NOT NULL,
 critical_projection_version INTEGER NOT NULL DEFAULT 1 CHECK(critical_projection_version=1),
 last_event_id TEXT, last_namespace_event_id TEXT, created_at TEXT NOT NULL
) STRICT;
CREATE TABLE schema_migrations(version INTEGER PRIMARY KEY, checksum TEXT NOT NULL, completed_at TEXT NOT NULL, backup_ref TEXT) STRICT;
CREATE TABLE storage_namespace (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), namespace_id TEXT NOT NULL UNIQUE, root TEXT NOT NULL UNIQUE,
 volume_identity TEXT NOT NULL, epoch INTEGER NOT NULL DEFAULT 0, row_revision INTEGER NOT NULL DEFAULT 0
) STRICT;
CREATE TABLE profiles (
 id TEXT PRIMARY KEY CHECK(length(id)=32), label TEXT NOT NULL, active_credential_revision_id TEXT,
 default_identity_revision_id TEXT, membership_generation INTEGER NOT NULL DEFAULT 0,
 desired_digest TEXT NOT NULL, row_revision INTEGER NOT NULL DEFAULT 0, retired_at TEXT,
 FOREIGN KEY(id,active_credential_revision_id) REFERENCES profile_credential_revisions(profile_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(id,default_identity_revision_id) REFERENCES profile_revisions(profile_id,id) DEFERRABLE INITIALLY DEFERRED,
 UNIQUE(id,active_credential_revision_id)
) STRICT;
CREATE UNIQUE INDEX profiles_label ON profiles(label) WHERE retired_at IS NULL;
CREATE TABLE profile_credential_revisions (
 id TEXT PRIMARY KEY CHECK(length(id)=32), profile_id TEXT NOT NULL REFERENCES profiles(id), ordinal INTEGER NOT NULL CHECK(ordinal>0),
 content_digest TEXT NOT NULL, public_key TEXT NOT NULL, fingerprint TEXT NOT NULL,
 key_ref TEXT NOT NULL CHECK(json_valid(key_ref)), key_ownership TEXT NOT NULL CHECK(key_ownership IN('owned','external')),
 agent_management TEXT NOT NULL CHECK(agent_management IN('owned','external')), agent_binding TEXT NOT NULL CHECK(json_valid(agent_binding)),
 store_ref TEXT CHECK(store_ref IS NULL OR json_valid(store_ref)), store_ownership TEXT CHECK(store_ownership IN('owned','external')),
 provider TEXT, expected_account TEXT, created_at TEXT NOT NULL,
 CHECK((store_ref IS NULL)=(store_ownership IS NULL)),
 UNIQUE(profile_id,ordinal), UNIQUE(profile_id,id), UNIQUE(profile_id,id,agent_management)
) STRICT;
CREATE TABLE profile_revisions (
 id TEXT PRIMARY KEY CHECK(length(id)=32), profile_id TEXT NOT NULL REFERENCES profiles(id), credential_revision_id TEXT NOT NULL,
 ordinal INTEGER NOT NULL CHECK(ordinal>0), guest_identity TEXT NOT NULL CHECK(json_valid(guest_identity) AND length(guest_identity)<=65536),
 content_digest TEXT NOT NULL, created_at TEXT NOT NULL,
 FOREIGN KEY(profile_id,credential_revision_id) REFERENCES profile_credential_revisions(profile_id,id),
 UNIQUE(profile_id,ordinal), UNIQUE(profile_id,id), UNIQUE(profile_id,id,credential_revision_id)
) STRICT;
CREATE TRIGGER credential_immutable BEFORE UPDATE ON profile_credential_revisions BEGIN SELECT RAISE(ABORT,'immutable credential revision'); END;
CREATE TRIGGER profile_revision_immutable BEFORE UPDATE ON profile_revisions BEGIN SELECT RAISE(ABORT,'immutable profile revision'); END;
CREATE TABLE registrations (
 id TEXT PRIMARY KEY CHECK(length(id)=32), namespace_id TEXT NOT NULL REFERENCES storage_namespace(namespace_id), instance_name TEXT NOT NULL,
 ssh_alias TEXT NOT NULL, source_path TEXT NOT NULL, source_key TEXT NOT NULL, profile_id TEXT REFERENCES profiles(id),
 applied_environment_revision_id TEXT, applied_profile_id TEXT, applied_profile_revision_id TEXT, applied_profile_binding_generation INTEGER,
 pending_identity_kind TEXT CHECK(pending_identity_kind IN('profile','none')), pending_profile_id TEXT, pending_profile_revision_id TEXT,
 identity_transition_phase TEXT CHECK(identity_transition_phase IN('preparing','committed')), identity_operation_id TEXT REFERENCES operations(id) DEFERRABLE INITIALLY DEFERRED,
 profile_binding_generation INTEGER NOT NULL DEFAULT 0, binding_generation INTEGER NOT NULL DEFAULT 0, owner_generation INTEGER NOT NULL DEFAULT 0,
 owner_event_id TEXT, publication_pending INTEGER NOT NULL DEFAULT 0 CHECK(publication_pending IN(0,1)),
 ownership TEXT NOT NULL CHECK(ownership IN('reserved','adopting','owned','retired','conflict','unknown')),
 lifecycle TEXT NOT NULL CHECK(lifecycle IN('planned','running','stopped','retired','unknown')),
 active_trust_generation INTEGER, pending_trust_operation_id TEXT REFERENCES operations(id),
 row_revision INTEGER NOT NULL DEFAULT 0, retired_at TEXT, created_at TEXT NOT NULL,
 FOREIGN KEY(id,applied_environment_revision_id) REFERENCES environment_revisions(registration_id,id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(applied_profile_id,applied_profile_revision_id) REFERENCES profile_revisions(profile_id,id),
 FOREIGN KEY(pending_profile_id,pending_profile_revision_id) REFERENCES profile_revisions(profile_id,id),
 FOREIGN KEY(id,active_trust_generation) REFERENCES vm_trust(registration_id,trust_generation) DEFERRABLE INITIALLY DEFERRED,
 CHECK((applied_profile_id IS NULL)=(applied_profile_revision_id IS NULL)),
 CHECK(applied_profile_id IS NULL OR applied_profile_binding_generation IS NOT NULL),
 CHECK((pending_identity_kind IS NULL AND pending_profile_id IS NULL AND pending_profile_revision_id IS NULL AND identity_transition_phase IS NULL AND identity_operation_id IS NULL)
 OR (pending_identity_kind='profile' AND pending_profile_id IS NOT NULL AND pending_profile_revision_id IS NOT NULL AND identity_transition_phase IS NOT NULL AND identity_operation_id IS NOT NULL)
 OR (pending_identity_kind='none' AND pending_profile_id IS NULL AND pending_profile_revision_id IS NULL AND identity_transition_phase IS NOT NULL AND identity_operation_id IS NOT NULL)),
 CHECK(identity_transition_phase IS NOT 'committed' OR (pending_identity_kind='profile' AND profile_id IS pending_profile_id) OR (pending_identity_kind='none' AND profile_id IS NULL)),
 UNIQUE(id,namespace_id,instance_name)
) STRICT;
CREATE UNIQUE INDEX registration_source ON registrations(source_key) WHERE retired_at IS NULL;
CREATE UNIQUE INDEX registration_instance ON registrations(namespace_id,instance_name) WHERE retired_at IS NULL;
CREATE UNIQUE INDEX registration_alias ON registrations(ssh_alias) WHERE retired_at IS NULL;
CREATE INDEX registration_profile ON registrations(profile_id);
CREATE TABLE environment_revisions (
 id TEXT PRIMARY KEY CHECK(length(id)=32), registration_id TEXT NOT NULL REFERENCES registrations(id),
 input_schema INTEGER NOT NULL CHECK(input_schema=1), content_digest TEXT NOT NULL, inputs TEXT NOT NULL CHECK(json_valid(inputs) AND length(inputs)<=65536),
 artifacts TEXT NOT NULL CHECK(json_valid(artifacts)), provision_revision TEXT NOT NULL, history TEXT NOT NULL CHECK(history IN('known','unknown')),
 created_at TEXT NOT NULL, UNIQUE(registration_id,id)
) STRICT;
CREATE TRIGGER environment_immutable BEFORE UPDATE ON environment_revisions BEGIN SELECT RAISE(ABORT,'immutable environment revision'); END;
CREATE TABLE operations (
 id TEXT PRIMARY KEY CHECK(length(id)=32), kind TEXT NOT NULL, scope_kind TEXT NOT NULL, scope_id TEXT, scope_path TEXT,
 parent_operation_id TEXT REFERENCES operations(id), retry_of_operation_id TEXT REFERENCES operations(id),
 token TEXT NOT NULL, status TEXT NOT NULL CHECK(status IN('prepared','active','needs_reconciliation','reconciling','blocked_known','completed','failed_known','cancelled_before_dispatch')),
 stop_reason TEXT, last_confirmed_stage TEXT, runtime_path TEXT, runtime_version TEXT,
 inputs TEXT NOT NULL CHECK(json_valid(inputs) AND length(inputs)<=65536), plan TEXT NOT NULL CHECK(json_valid(plan) AND length(plan)<=65536),
 requested_postconditions TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(requested_postconditions)), ancillary_checks TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(ancillary_checks)),
 user_decisions TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(user_decisions)), created_at TEXT NOT NULL, updated_at TEXT NOT NULL,
 CHECK(parent_operation_id IS NOT id AND retry_of_operation_id IS NOT id)
) STRICT;
CREATE INDEX operation_active ON operations(status,scope_id);
CREATE TABLE operation_steps (
 operation_id TEXT NOT NULL REFERENCES operations(id), stage TEXT NOT NULL, attempt INTEGER NOT NULL CHECK(attempt>0),
 dispatch_state TEXT NOT NULL CHECK(dispatch_state IN('none','may_have_dispatched','exited')),
 effect_state TEXT NOT NULL CHECK(effect_state IN('satisfied','active','not_applied','partial','unknown')),
 evidence TEXT NOT NULL CHECK(json_valid(evidence) AND length(evidence)<=65536), observed_at TEXT NOT NULL, result_code TEXT NOT NULL,
 PRIMARY KEY(operation_id,stage,attempt)
) STRICT;
CREATE TABLE resource_claims(operation_id TEXT NOT NULL REFERENCES operations(id),kind TEXT NOT NULL,resource_key TEXT NOT NULL,mode TEXT NOT NULL CHECK(mode IN('shared','exclusive')),token TEXT NOT NULL,PRIMARY KEY(operation_id,kind,resource_key)) STRICT;
CREATE INDEX claims_resource ON resource_claims(kind,resource_key);
CREATE TRIGGER claim_conflict BEFORE INSERT ON resource_claims WHEN EXISTS(SELECT 1 FROM resource_claims WHERE kind=NEW.kind AND resource_key=NEW.resource_key AND operation_id<>NEW.operation_id AND (mode='exclusive' OR NEW.mode='exclusive')) BEGIN SELECT RAISE(ABORT,'conflicting operation claim'); END;
CREATE TABLE binding_reservations(operation_id TEXT NOT NULL REFERENCES operations(id),registration_id TEXT NOT NULL REFERENCES registrations(id),source_key TEXT NOT NULL,source_path TEXT NOT NULL,profile_id TEXT REFERENCES profiles(id),profile_present INTEGER NOT NULL CHECK(profile_present IN(0,1)),PRIMARY KEY(operation_id,registration_id)) STRICT;
CREATE UNIQUE INDEX reservation_source ON binding_reservations(source_key);
CREATE TABLE owned_settings(owner_id TEXT NOT NULL,owner_kind TEXT NOT NULL,target TEXT NOT NULL,field TEXT NOT NULL,expected_digest TEXT NOT NULL,applied_revision TEXT,backup_ref TEXT,PRIMARY KEY(owner_id,target,field)) STRICT;
CREATE TABLE binding_changes(operation_id TEXT NOT NULL REFERENCES operations(id),registration_id TEXT NOT NULL REFERENCES registrations(id),kind TEXT NOT NULL,old_binding TEXT NOT NULL CHECK(json_valid(old_binding)),proposed_binding TEXT NOT NULL CHECK(json_valid(proposed_binding)),backups TEXT NOT NULL DEFAULT '[]' CHECK(json_valid(backups)),stage TEXT NOT NULL,PRIMARY KEY(operation_id,registration_id)) STRICT;
CREATE TABLE inventory_exports(registration_id TEXT PRIMARY KEY REFERENCES registrations(id),desired_generation INTEGER NOT NULL,payload TEXT NOT NULL CHECK(json_valid(payload)),digest TEXT NOT NULL,published_generation INTEGER,published_digest TEXT,last_error TEXT) STRICT;
CREATE TABLE profile_runtime (
 profile_id TEXT PRIMARY KEY REFERENCES profiles(id),credential_revision_id TEXT NOT NULL,agent_management TEXT NOT NULL CHECK(agent_management IN('owned','external')),
 generation INTEGER NOT NULL DEFAULT 0,lock_state TEXT CHECK(lock_state IN('locked','unlocked')),remember_enabled INTEGER NOT NULL CHECK(remember_enabled IN(0,1)),
 transition_operation_id TEXT REFERENCES operations(id),service_evidence TEXT CHECK(service_evidence IS NULL OR json_valid(service_evidence)),
 last_verified_fingerprint TEXT,last_verified_at TEXT,last_result_code TEXT,row_revision INTEGER NOT NULL DEFAULT 0,
 FOREIGN KEY(profile_id,credential_revision_id) REFERENCES profiles(id,active_credential_revision_id) DEFERRABLE INITIALLY DEFERRED,
 FOREIGN KEY(profile_id,credential_revision_id,agent_management) REFERENCES profile_credential_revisions(profile_id,id,agent_management),
 CHECK((agent_management='external' AND lock_state IS NULL) OR (agent_management='owned' AND lock_state IS NOT NULL)),
 CHECK((last_verified_at IS NULL)=(last_verified_fingerprint IS NULL))
) STRICT;
CREATE TABLE vm_trust (
 registration_id TEXT NOT NULL,trust_generation INTEGER NOT NULL,namespace_id TEXT NOT NULL,instance_name TEXT NOT NULL,
 algorithm TEXT NOT NULL,public_key TEXT NOT NULL,fingerprint TEXT NOT NULL,source TEXT NOT NULL CHECK(source IN('imported_pin','local_first_use','explicit_repair')),
 enrollment_operation_id TEXT NOT NULL REFERENCES operations(id),evidence_digest TEXT NOT NULL,created_at TEXT NOT NULL,
 PRIMARY KEY(registration_id,trust_generation),FOREIGN KEY(registration_id,namespace_id,instance_name) REFERENCES registrations(id,namespace_id,instance_name)
) STRICT;
CREATE TABLE connection_bindings (
 id TEXT PRIMARY KEY,registration_id TEXT NOT NULL REFERENCES registrations(id),profile_id TEXT NOT NULL,credential_revision_id TEXT NOT NULL,identity_revision_id TEXT NOT NULL,
 generation INTEGER NOT NULL,profile_binding_generation INTEGER NOT NULL,profile_runtime_generation INTEGER NOT NULL,trust_generation INTEGER NOT NULL,
 vm_boot_id TEXT,guest_uid INTEGER NOT NULL CHECK(guest_uid>=0),host_agent_socket TEXT NOT NULL,host_control_socket TEXT NOT NULL,guest_socket TEXT NOT NULL,
 endpoint_address TEXT NOT NULL CHECK(endpoint_address IN('127.0.0.1','::1')),endpoint_port INTEGER NOT NULL CHECK(endpoint_port BETWEEN 1 AND 65535),
 state TEXT NOT NULL CHECK(state IN('prepared','establishing','ready','revoking','unknown','retired')),
 operation_id TEXT NOT NULL REFERENCES operations(id),process_evidence TEXT,last_probe TEXT,created_at TEXT NOT NULL,retired_at TEXT,row_revision INTEGER NOT NULL DEFAULT 0,
 FOREIGN KEY(profile_id,identity_revision_id,credential_revision_id) REFERENCES profile_revisions(profile_id,id,credential_revision_id),
 FOREIGN KEY(registration_id,trust_generation) REFERENCES vm_trust(registration_id,trust_generation),
 CHECK((state='retired')=(retired_at IS NOT NULL)), CHECK(state<>'ready' OR vm_boot_id IS NOT NULL)
) STRICT;
CREATE UNIQUE INDEX live_connection ON connection_bindings(registration_id) WHERE state<>'retired';
CREATE UNIQUE INDEX live_control ON connection_bindings(host_control_socket) WHERE state<>'retired';
CREATE INDEX connection_profile ON connection_bindings(profile_id,state);
CREATE TABLE capacity_claims(operation_id TEXT NOT NULL REFERENCES operations(id),capacity_group_id TEXT NOT NULL,phase_id TEXT NOT NULL,namespace_id TEXT NOT NULL REFERENCES storage_namespace(namespace_id),destination_ledger_digest TEXT NOT NULL,filesystem_evidence TEXT NOT NULL CHECK(json_valid(filesystem_evidence)),peak_bytes INTEGER NOT NULL CHECK(peak_bytes>=0),remaining_bytes INTEGER NOT NULL CHECK(remaining_bytes>=0 AND remaining_bytes<=peak_bytes),state TEXT NOT NULL CHECK(state IN('held','released')),admitted_generation INTEGER NOT NULL,released_at TEXT,PRIMARY KEY(operation_id,capacity_group_id,phase_id),CHECK(state<>'released' OR (remaining_bytes=0 AND released_at IS NOT NULL))) STRICT;
CREATE TABLE namespace_events (
 id TEXT PRIMARY KEY,sequence INTEGER NOT NULL UNIQUE CHECK(sequence>0),operation_id TEXT NOT NULL,namespace_id TEXT REFERENCES storage_namespace(namespace_id),kind TEXT NOT NULL,
 previous_event_id TEXT,previous_event_hash TEXT,previous_namespace_event_id TEXT,before_digest TEXT NOT NULL,after_digest TEXT NOT NULL,
 payload TEXT NOT NULL CHECK(json_valid(payload) AND length(payload)<=65536),event_hash TEXT NOT NULL,
 publication_state TEXT NOT NULL CHECK(publication_state IN('pending_local','pending_namespace','published','conflict')),
 created_at TEXT NOT NULL,local_published_at TEXT,namespace_published_at TEXT
) STRICT;
CREATE INDEX event_pending ON namespace_events(publication_state,sequence);
