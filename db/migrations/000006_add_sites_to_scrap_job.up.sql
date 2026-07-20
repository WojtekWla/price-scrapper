ALTER TABLE scrap_job 
ADD COLUMN sites text[] NOT NULL DEFAULT '{}';