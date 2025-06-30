ALTER TABLE userSettings ADD COLUMN inprogress TEXT DEFAULT '';
ALTER TABLE userSettings ADD COLUMN processed_count INT DEFAULT 1;