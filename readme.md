# GoMultiRate

GoMultiRate is a rate limiter designed for multiple use cases, especially for multiple rate limit windows.

The initial motivation for development was the Riot API, which usually holds two different limits in different spans of time.

## Table of contents
- [GoMultiRate](#gomultirate)
  - [Table of contents](#table-of-contents)
  - [Features](#features)
  - [Installation](#installation)
  - [Usage](#usage)
  - [Try](#try)
  - [Wait](#wait)
  - [WaitEvenly](#waitevenly)
  - [Concurrency](#concurrency)
  - [License](#license)
## Features

- Implements Token Buckets and Leaky Buckets.
- Thread-safe with usage of mutex.
- Configurable rate limit creation with multiple limits.

## Installation

```bash
go get github.com/Gustavo-Feijo/gomultirate
```

## Usage
After importing the package, a map of limits must be created, providing the limit key and creating a new limit with the interval and limit count.

The map of limits will be used to create the rate limiter, that will provide the following methods:
 ```Wait()``` - Wait until all the limits are available again.
 ```Try()``` - Try to get the limits in a non blocking way.
 ```WaitEvenly``` - Leaky Bucket implementation, wait at a constant rate. Receives a key as parameter to define which constant rate to use.

 The wait methods require context to be passed.
 
 The try method returns a bool indicating success and how much time until the next reset if it wasn't successful.

## Try

Below a simple example of the limiter creation and usage of the Try method.

```go
package main

import (
    "fmt"
    "time"
    "github.com/Gustavo-Feijo/gomultirate"
)

func main() {
    // Create the limits map.
    limits := map[string]*gomultirate.Limit{
    	"slow": gomultirate.NewLimit(time.Minute, 10),
    	"fast": gomultirate.NewLimit(5*time.Second, 2),
    }  

    limiter, err := gomultirate.NewRateLimiter(limits)
    if err != nil {
        // Handle error.
        // Should only occur if empty map is passed.
    }  

    for range 3{
        limiter.Try()
        // True
        // True
        // False, since the "fast" limit was reached.
    }
}
```
## Wait
Using the same limiter as above, but now with the Wait() method:

```go
    start := time.Now()
    ctx := context.Background()
    for range 10 {
    	limiter.Wait(ctx)
    	fmt.Println(time.Since(start))
        //1.402µs
        //28.585µs
        //5.003607602s
        //5.003640791s
        //10.007756132s
        //10.00778023s
        //15.011644726s
        //15.011688405s
        //20.015621301s
        //20.015662208s
    }
```

## WaitEvenly
Now, using the WaitEvenly() with the "fast" key, which implements the leaky bucket:
```go
    start := time.Now()
    ctx := context.Background()
    for range 10 {
    	limiter.WaitEvenly(ctx, "fast")
    	fmt.Println(time.Since(start))
        //940ns
        //2.500645756s
        //5.00169858s
        //7.502860827s
        //10.002743906s
        //12.503810772s
        //15.003654443s
        //17.504633779s
        //20.005229955s
        //22.506668078s
    }
```
## Concurrency

Working with concurrency is also pretty straight forward:
```go
    start := time.Now()
    ctx := context.Background() 
    
    // Create the wait group and the channel.
    var wg sync.WaitGroup
    values := make(chan int, 10)    
    
    // Create 3 workers that will consume from the channel.
    const workers = 3
    for range workers {
    	go func() {
    	    for i := range values {
    	        limiter.WaitEvenly(ctx, "fast")
    	        fmt.Printf("Processing value %d at %v\n", i, time.Since(start))
    	        wg.Done()
    	    }
    	}()
    }   
    // Add 10 to the channel.
    for i := range 10 {
    	wg.Add(1)
    	values <- i
    }   
    // Close the channel and wait.
    close(values)
    wg.Wait()
    
    //Processing value 0 at 28.183µs
    //Processing value 1 at 2.501461664s
    //Processing value 3 at 5.001480464s
    //Processing value 4 at 7.50248908s
    //Processing value 6 at 10.002497763s
    //Processing value 5 at 12.504046172s
    //Processing value 2 at 15.003516874s
    //Processing value 8 at 17.50518334s
    //Processing value 7 at 20.005482485s
    //Processing value 9 at 22.506500163s
```
## License

This project is licensed under the [MIT License](LICENSE).