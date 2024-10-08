CREATE TABLE message_replies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id INTEGER NOT NULL,
    tg_chat_id INTEGER NOT NULL,
    tg_pen_id INTEGER NOT NULL,
    reply_timestamp TIMESTAMP,
    reply_type TEXT NOT NULL
);

CREATE INDEX idx_message_replies_tg_pen_id ON message_replies(tg_pen_id);