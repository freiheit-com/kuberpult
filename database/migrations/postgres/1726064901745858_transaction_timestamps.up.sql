-- Table that stores which cd-service transaction timestamp corresponds to a certain commit hash
-- on the manifest repo
CREATE TABLE IF NOT EXISTS commit_transaction_timestamps
(
    commitHash VARCHAR NOT NULL,
    transactionTimestamp TIMESTAMP NOT NULL,
    PRIMARY KEY(commitHash)
);
