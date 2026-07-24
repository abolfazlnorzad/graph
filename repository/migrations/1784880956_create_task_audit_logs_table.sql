-- +migrate Up

CREATE TABLE IF NOT EXISTS task_audit_logs (
                                               id BIGSERIAL PRIMARY KEY,
                                               task_id BIGINT NOT NULL,
                                               action VARCHAR(50) NOT NULL,
                                               previous_state JSONB,
                                               new_state JSONB,
                                               created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,

                                               CONSTRAINT fk_audit_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
    );

CREATE INDEX idx_audit_logs_task_id ON task_audit_logs(task_id);
CREATE INDEX idx_audit_logs_created_at ON task_audit_logs(created_at DESC);



-- +migrate Down
DROP INDEX IF EXISTS idx_audit_logs_created_at;
DROP INDEX IF EXISTS idx_audit_logs_task_id;
DROP TABLE IF EXISTS task_audit_logs;
