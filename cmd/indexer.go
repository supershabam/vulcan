// Copyright 2016 The Vulcan Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/digitalocean/vulcan/elasticsearch"
	"github.com/digitalocean/vulcan/indexer"
	"github.com/digitalocean/vulcan/kafka"

	"github.com/olivere/elastic"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Indexer handles parsing the command line options, initializes, and starts the
// indexer service accordingling.  It is the entry point for the Indexer
// service.
func Indexer() *cobra.Command {
	var Indexer = &cobra.Command{
		Use:   "indexer",
		Short: "consumes metrics from the bus and makes them searchable",
		RunE: func(cmd *cobra.Command, args []string) error {
			// get kafka source
			s, err := kafka.NewSource(&kafka.SourceConfig{
				Addrs:    strings.Split(viper.GetString(flagKafkaAddrs), ","),
				ClientID: viper.GetString(flagKafkaClientID),
				GroupID:  viper.GetString(flagKafkaGroupID),
				Topics:   []string{viper.GetString(flagKafkaTopic)},
			})
			if err != nil {
				return err
			}

			// custom client used so we can more effeciently reuse connections.
			customClient := &http.Client{
				Transport: &http.Transport{
					Dial: func(network, addr string) (net.Conn, error) {
						return net.Dial(network, addr)
					},
					MaxIdleConnsPerHost: viper.GetInt("max-idle-conn"),
				},
			}

			// allow sniff to be set because in some networking environments sniffing doesn't work. Should be allowed in prod
			client, err := elastic.NewClient(
				elastic.SetURL(viper.GetString("es")),
				elastic.SetSniff(viper.GetBool("es-sniff")),
				elastic.SetHttpClient(customClient),
			)
			if err != nil {
				return err
			}

			// set up caching es sample indexer
			esIndexer := elasticsearch.NewSampleIndexer(&elasticsearch.SampleIndexerConfig{
				Client: client,
				Index:  viper.GetString("es-index"),
			})
			sampleIndexer := indexer.NewCachingIndexer(&indexer.CachingIndexerConfig{
				Indexer:     esIndexer,
				MaxDuration: viper.GetDuration("es-writecache-duration"),
			})

			// create indexer and run
			i := indexer.NewIndexer(&indexer.Config{
				SampleIndexer:      sampleIndexer,
				Source:             s,
				NumIndexGoroutines: viper.GetInt("indexer-goroutines"),
			})

			prometheus.MustRegister(i)
			prometheus.MustRegister(esIndexer)
			prometheus.MustRegister(sampleIndexer)

			go func() {
				http.Handle("/metrics", prometheus.Handler())
				http.ListenAndServe(":8080", nil)
			}()

			return i.Run()
		},
	}

	Indexer.Flags().String(flagKafkaAddrs, "", "one.example.com:9092,two.example.com:9092")
	Indexer.Flags().String(flagKafkaClientID, "vulcan-indexer", "set the kafka client id")
	Indexer.Flags().String(flagKafkaTopic, "vulcan", "topic to read in kafka")
	Indexer.Flags().String(flagKafkaGroupID, "vulcan-indexer", "workers with the same groupID will join the same Kafka ConsumerGroup")
	Indexer.Flags().String("es", "http://elasticsearch:9200", "elasticsearch connection url")
	Indexer.Flags().Bool("es-sniff", true, "whether or not to sniff additional hosts in the cluster")
	Indexer.Flags().String("es-index", "vulcan", "the elasticsearch index to write documents into")
	Indexer.Flags().Duration("es-writecache-duration", time.Minute*10, "the duration to cache having written a value to es and to skip further writes of the same metric")
	Indexer.Flags().Uint("indexer-goroutines", 30, "worker goroutines for writing indexes")
	Indexer.Flags().Uint("max-idle-conn", 30, "max idle connections for fetching from data storage")

	return Indexer
}
