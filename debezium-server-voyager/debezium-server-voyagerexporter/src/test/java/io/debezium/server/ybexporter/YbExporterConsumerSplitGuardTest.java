/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.List;

import org.apache.kafka.connect.errors.ConnectException;
import org.junit.jupiter.api.Test;

import io.debezium.engine.ChangeEvent;
import io.debezium.engine.DebeziumEngine;

/**
 * Plain JUnit 5 unit test (NOT a @QuarkusTest) exercising the tablet-split offset-commit
 * guard in {@link YbExporterConsumer#commitBatchOffsets}. The guard must swallow ONLY the
 * "replication stream has been closed" exception (which a YB tablet split triggers during
 * the offset flush) and rethrow anything else.
 */
public class YbExporterConsumerSplitGuardTest {

    /**
     * Fake RecordCommitter whose markBatchFinished() throws a supplied exception. All other
     * methods are no-ops so commitBatchOffsets exercises only the offset-commit path.
     */
    private static final class FakeCommitter implements DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> {
        private final RuntimeException toThrow;

        FakeCommitter(RuntimeException toThrow) {
            this.toThrow = toThrow;
        }

        @Override
        public void markProcessed(ChangeEvent<Object, Object> record) {
            // no-op
        }

        @Override
        public void markBatchFinished() {
            throw toThrow;
        }

        @Override
        public void markProcessed(ChangeEvent<Object, Object> record, DebeziumEngine.Offsets sourceOffsets) {
            // no-op
        }

        @Override
        public DebeziumEngine.Offsets buildOffsets() {
            return null;
        }
    }

    private YbExporterConsumer newConsumer() {
        // Constructor only stores dataDir; do NOT call connect().
        return new YbExporterConsumer("/tmp");
    }

    @Test
    public void swallowsDirectReplicationStreamClosed() {
        YbExporterConsumer consumer = newConsumer();
        FakeCommitter committer = new FakeCommitter(
                new ConnectException("This replication stream has been closed"));

        assertThatCode(() -> consumer.commitBatchOffsets(List.of(), committer))
                .doesNotThrowAnyException();
    }

    @Test
    public void swallowsNestedReplicationStreamClosed() {
        YbExporterConsumer consumer = newConsumer();
        FakeCommitter committer = new FakeCommitter(
                new RuntimeException("wrap",
                        new ConnectException("... This replication stream has been closed ...")));

        assertThatCode(() -> consumer.commitBatchOffsets(List.of(), committer))
                .doesNotThrowAnyException();
    }

    @Test
    public void rethrowsUnrelatedFailure() {
        YbExporterConsumer consumer = newConsumer();
        RuntimeException unrelated = new RuntimeException("some unrelated failure");
        FakeCommitter committer = new FakeCommitter(unrelated);

        assertThatThrownBy(() -> consumer.commitBatchOffsets(List.of(), committer))
                .isSameAs(unrelated);
        assertThat(unrelated).hasMessage("some unrelated failure");
    }
}
