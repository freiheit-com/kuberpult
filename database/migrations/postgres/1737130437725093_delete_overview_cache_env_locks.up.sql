-- We changed the overview structure. As such, we need to recalculate the overview
INSERT INTO overview_cache (timestamp, json) VALUES (now(), '{}')