# GoChop.it 
A Go based URL shortener

## Tech Stack:
- Go
- Redis (caching)
- MongoDB (persistent db)
- HTMX 

## Proposed Architecture 
```
            +---------------------+
            |     User Requests   |
            +---------------------+
                      |
                      v
         +----------------------------+
         |   Load Balancer (Optional) |  
         +----------------------------+
                      |
                      v
         +----------------------------+
         |      URL Shortener API     |  
         |        (Go Service)        |
         +----------------------------+
                      |
                      v
  +------------------------------------------+
  |          Caching Layer (Redis)           |
  +------------------------------------------+
                      |
                      v
  +------------------------------------------+
  |       Persistent Storage (MongoDB)       |
  +------------------------------------------+
```

### references
- https://stackoverflow.com/questions/742013/how-do-i-create-a-url-shortener

- https://bitly.com/blog/how-to-make-a-url-shortener/