# karmafarm
A data pipeline to farm [statistics about] karma

## Basic functioning
The software is designed to run continuously, reading input lines from a file as it's being written. Each of the lines
read contains details about an independent event, or `finding`, and is immediately queued up for insertion into a
CouchDB database. The application takes also care of adding the appropriate view functions to the database itself,
so that the processed data can be consulted as it gets inserted.

The database gets wiped every time the application is run, and data already present in the input file is parsed as well.
The database can be accessed via its web interface at http://localhost:5984/_utils/ using `admin` and `pass` as username
and password (set via the `.ini` file), but it also offers a whole set of REST endpoints for accessing the same data
programmatically. Specifically, as the event database is called `finding`, its data can be fetched with
`curl 'http://127.0.0.1:5984/finding/_all_docs?endkey="_"&include_docs=true'`, where the `endkey` parameter is used to
skip fetching the design documents too (i.e. the views), which are indeed stored alongside the actual data.

## Results

The processed information, i.e. the amount of karma each croudsourcer has earned, is available via the custom view at
`http://127.0.0.1:5984/finding/_design/findingDesign/_view/findingcs?group_level=4` (in JSON format), where the
`group_level` parameter can vary between 0 and 5 to select how many keys should be used to group the results;
they are [`crowdsourcer's name`, `severity of the vulnerability`, `year`, `month`, `day`], and for any choice the
relative total amount of karma is calculated.

The results are available also in a simple HTML page at
`http://127.0.0.1:5984/finding/_design/findingDesign/_list/contributors/findingcs?group_level=4`

## Installation and Execution

The `docker-compose` file available in the `compose` folder should be all that is needed to compile and run the code:
issuing the `docker-compose build` and `docker-compose run` commands should perform both those operations. Alternatively,
for manual execution, having the `go` binary available and the `GOPATH` environment variable pointing at the `solution`
folder should be enough to compile and run the program by typing `go run reader.go` from within the `src/karmafarm`
folder. The database will need to be available at the URL specified with the `COUCHDB_URL` environment variable (or at
`http://admin:pass@localhost:5984/` if undefined), and the `input` folder should be either a sibling of `solution`, or
specified by the `INPUT_LOCATION` environment variable. The complete list of environment variables available for tuning
is in the `.env` file.

## Assumptions

- Only the `finding.csv` file is assumed to be subject to changes; the other ones are not monitored for new data.
- The `id` field is considered to be primary key, so `findings` with the same `id` are going to be considered updates.

## Discussion points

- The program keeps track of successful input reads, so that in the event of a failed one (which can happen, with
concurrent read and write operations) it can backtrack to a known valid offset in the file; entries with the same `id`
are checked for equality, so that updates happen only when the data differs; if the writing to the input file gets halted,
the program simply waits until new input is foud; the database is not checked for availability, because making sure it's
up and running without polling it constantly would have required a more complex architecture – namely an additional
communication channel for handling the waiting tasks, and yet another channel for storing the messages that would otherwise be lost.
- This solution is essentially limited by the amount of available RAM and the read/write speed of the input file:
go threads, or more appropriately goroutines, are very cheap to spawn and delegate some work to, so their amount cap
can be adjusted if input events are too frequent. As per the default values, at least 1 and up to 30 threads can
be running concurrently, each delegated to PUT an event into the database; their number increases when the message
queue gets full (default max size is 100), and it decreases as the file read operations return EOF errors, i.e. the file
is being read essentially in real time as it's being written. The database is also extendable with more instances running
in a cluster, if that becomes a bottleneck.
- Filtering the input data for the view would require a different implementation of a document-oriented database, one that
would allow chaining view functions; CouchDB has had the feature in their roadmap for a while now, but it seems like it's
not ready yet.
- Leveraging the power of map/reduce operations, the change needed to get an `average` field together with the accumulated
karma as the result would be to calculate it as the entries get aggregated, so changing the `reduce` function to handle
an array of values instead of just one, to keep track and update the sum, the average, and the number of events.
- The biggest trade off made here is speed of data processing in exchange for fine filtering capabilities: in this instance
it's a good trade, because each row of data is completely independent and does not require updating over time.