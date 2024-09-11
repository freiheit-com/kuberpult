-- Table that stores which timestamp
CREATE TABLE IF NOT EXISTS commit_transaction_timestamps
(
    commitHash VARCHAR NOT NULL,
    transactionTimestamp TIMESTAMP NOT NULL,
    PRIMARY KEY(commitHash)
);
