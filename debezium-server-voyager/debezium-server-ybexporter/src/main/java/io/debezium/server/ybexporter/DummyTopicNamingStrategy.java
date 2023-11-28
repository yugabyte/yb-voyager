/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.util.Properties;

import io.debezium.schema.DefaultTopicNamingStrategy;
import io.debezium.spi.schema.DataCollectionId;

public class DummyTopicNamingStrategy extends DefaultTopicNamingStrategy {

    public DummyTopicNamingStrategy(Properties props) {
        super(props);
    }

    @Override
    public String dataChangeTopic(DataCollectionId id) {
        return "topic";
    }
}