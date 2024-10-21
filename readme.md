# GoChop.it 
A Go based URL shortener

## Tech Stack:
- Go
- Redis (caching)
- MongoDB (persistent db)
- HTMX 

## Proposed Final Architecture 
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

## MVP Architecture 
```
            +---------------------+
            |     User Requests   |
            +---------------------+
                      |
                      v
         +----------------------------+
         |      URL Shortener API     |  
         |        (Go Service)        |
         +----------------------------+
                      |
                      v
  +------------------------------------------+
  |            Redis as a DB                 |
  +------------------------------------------+
```

### CI / CD
#### Pre-Commit (Local)
- **Husky**
  - Used to catch basic formatting, linting, and test failures before code is even committed. 
  - This can be bypassed if necessary but act as a first line of defense.

#### GitHub Actions


### references
- https://stackoverflow.com/questions/742013/how-do-i-create-a-url-shortener

- https://bitly.com/blog/how-to-make-a-url-shortener/

- https://getstream.io/blog/url-shortener/ mvp

### TODO
- [x] pre commit hooks https://bongnv.com/blog/2021-08-29-pre-commit-hooks-golang-projects/
- [x] testing 
- [ ] persistent storage 
- [ ] caching layer 
- [ ] better shortener algo
- [ ] deployment