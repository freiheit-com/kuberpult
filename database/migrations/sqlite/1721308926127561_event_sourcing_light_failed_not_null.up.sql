-- Create a new table with the desired constraints
CREATE TABLE event_sourcing_light_failed_new
(
    eslId INTEGER PRIMARY KEY AUTOINCREMENT,
    created TIMESTAMP NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    json VARCHAR NOT NULL
);

-- Copy the data from the old table to the new table
INSERT INTO event_sourcing_light_failed_new (eslId, created, event_type, json)
SELECT eslId, created, event_type, json
FROM event_sourcing_light_failed;

-- Drop the old table
DROP TABLE event_sourcing_light_failed;

-- Rename the new table to the old table's name
ALTER TABLE event_sourcing_light_failed_new RENAME TO event_sourcing_light_failed;