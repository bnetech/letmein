-- DROP TABLE submissions;
-- CREATE TABLE submissions (
--     SubmissionID SERIAL PRIMARY KEY NOT NULL,
--     IPAddress VARCHAR(100) NULL,
--     Email VARCHAR(200) NULL,
--     PageOpened TIMESTAMP NOT NULL,
--     FormSubmitted TIMESTAMP NOT NULL,
--     Honeypot VARCHAR(500));

-- ALTER TABLE help ADD COLUMN Approved BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE help ADD COLUMN ApprovedBy VARCHAR NULL;

-- CREATE TABLE help (
--     HelpId SERIAL PRIMARY KEY NOT NULL,
--     HelpText TEXT NOT NULL,
--     HelpContact VARCHAR(500) NOT NULL,
--     PageOpened TIMESTAMP NOT NULL,
--     FormSubmitted TIMESTAMP NOT NULL
-- );
