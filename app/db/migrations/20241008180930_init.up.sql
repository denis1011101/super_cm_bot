CREATE TABLE IF NOT EXISTS pens (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pen_name TEXT,
    tg_pen_id INTEGER UNIQUE,
    tg_chat_id INTEGER,
    pen_length INTEGER,
    pen_last_update_at TIMESTAMP,
    handsome_count INTEGER,
    handsome_last_update_at TIMESTAMP,
    unhandsome_count INTEGER,
    unhandsome_last_update_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_pen_length ON pens(pen_length);

CREATE INDEX IF NOT EXISTS idx_tg_pen_id ON pens(tg_pen_id);