ALTER TABLE presigned_urls 
DROP CONSTRAINT presigned_urls_file_id_fkey;

ALTER TABLE presigned_urls
ADD CONSTRAINT presigned_urls_file_id_fkey 
FOREIGN KEY (file_id) REFERENCES files(id);
