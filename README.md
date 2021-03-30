# Wiki

Stores data in git

Supports basic auth for editing from either CLI arguments
   - -authrealm 
   - -authusername 
   - -authpassword

or from environment variables 
   - AUTHREALM 
   - AUTHUSERNAME 
   - AUTHPASSWORD

All paths are relative to the working directory, in the container this is /

 - <working directory>/data - Used to store data
 - <working directory>/templates - Used if you want to overwrite all templates
 - <working directory>/static - Used if you want to overwrite all static content

Docker container
 - working directory is /
 - runs as user 65532:65532