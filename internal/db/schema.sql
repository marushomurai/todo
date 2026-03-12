CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'done', 'cancelled')),
    done_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS daily_plans (
    plan_date TEXT NOT NULL PRIMARY KEY,
    state TEXT NOT NULL DEFAULT 'open' CHECK(state IN ('open', 'closed', 'reviewed')),
    closed_at DATETIME,
    reviewed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now', 'localtime'))
);

CREATE TABLE IF NOT EXISTS daily_plan_items (
    plan_date TEXT NOT NULL,
    task_id INTEGER NOT NULL,
    position INTEGER NOT NULL,
    disposition TEXT NOT NULL DEFAULT 'planned' CHECK(disposition IN ('planned', 'done', 'carried_over')),
    PRIMARY KEY (plan_date, task_id),
    UNIQUE (plan_date, position),
    FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_daily_plan_items_task ON daily_plan_items(task_id);
