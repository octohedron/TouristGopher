# TouristGopher ![gopher](http://touristgopher.com/images/android-icon-36x36.png)

> This gopher knows

See it in action at [touristgopher.com](http://touristgopher.com)

This clever Gopher combines your search for delicious food with results from Yelp, Google Places and FourSquare into their bayesian ranking estimates and puts them on a map

## Try it out
+ Install redis
+ Install dependencies
```Bash
$ go get
```
+ Set environment variables

```Bash
$ export GMAPS_KEY=YOUR_GMAPS_KEY
```
+ Run it
```Bash
$ go build && nohup ./TouristGopher & 
```

Check the other repo, [TouristFriend](http://github.com/octohedron/TouristFriend)

CONTRIBUTING: YES

LICENSE: MIT
