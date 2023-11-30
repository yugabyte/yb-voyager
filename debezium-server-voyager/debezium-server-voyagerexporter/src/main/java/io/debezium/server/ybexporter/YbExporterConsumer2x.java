/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import io.debezium.engine.ChangeEvent;
import io.debezium.engine.DebeziumEngine;
import io.debezium.server.BaseChangeConsumer;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import jakarta.annotation.PostConstruct;
import jakarta.enterprise.context.Dependent;
import jakarta.inject.Named;
import java.net.URISyntaxException;
import java.util.List;

/**
 * Implementation of the consumer that exports the messages to file in a Yugabyte-compatible form.
 */
@Named("ybexporter")
@Dependent
public class YbExporterConsumer2x extends BaseChangeConsumer implements DebeziumEngine.ChangeConsumer<ChangeEvent<Object, Object>> {
    private static final Logger LOGGER = LoggerFactory.getLogger(YbExporterConsumer2x.class);
    private static final String PROP_PREFIX = "debezium.sink.ybexporter.";
    @ConfigProperty(name = PROP_PREFIX + "dataDir")
    String dataDir;

    private YbExporterConsumer ybec;

    @PostConstruct
    void connect() throws URISyntaxException {
        ybec = new YbExporterConsumer(dataDir);
        ybec.connect();
    }



    @Override
    public void handleBatch(List<ChangeEvent<Object, Object>> changeEvents, DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> committer)
            throws InterruptedException {
        ybec.handleBatch(changeEvents, committer);
    }
}
