-- 011 down: drop audit_archive. The archive is cold storage;
-- operators should drain it before dropping. The migration does
-- not enforce this.

DROP TABLE audit_archive;
