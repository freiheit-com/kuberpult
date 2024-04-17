CREATE TABLE IF NOT EXISTS dummy_table
(   id BIGINT,
    created TIMESTAMP,
    data VARCHAR(255),
    PRIMARY KEY(id)
);

INSERT INTO dummy_table (id , created , data)  VALUES (0, 	"1709627400", "First Message");
INSERT INTO dummy_table (id , created , data)  VALUES (1, 	"1709627400", "Second Message");