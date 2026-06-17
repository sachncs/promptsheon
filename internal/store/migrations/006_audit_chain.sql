-- Migration 006: Add hash chaining to audit_entries for tamper evidence.
ALTER TABLE audit_entries ADD COLUMN previous_hash TEXT DEFAULT '';
ALTER TABLE audit_entries ADD COLUMN entry_hash TEXT DEFAULT '';
