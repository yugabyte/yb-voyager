/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.io.BufferedWriter;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.FileWriter;
import java.io.IOException;
import java.io.SyncFailedException;
import java.util.ArrayList;

import org.apache.commons.csv.CSVFormat;
import org.apache.commons.csv.CSVPrinter;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class TableSnapshotWriterCSV implements RecordWriter {
    private static final Logger LOGGER = LoggerFactory.getLogger(TableSnapshotWriterCSV.class);
    private ExportStatus es;
    private String sourceType;
    private String dataDir;
    private Table t;
    private CSVPrinter csvPrinter;
    private FileOutputStream fos;
    private FileDescriptor fd;
    private String ybVoyagerNullString = "__YBV_NULL__";

    public TableSnapshotWriterCSV(String datadirStr, Table tbl, String sourceType) {
        dataDir = datadirStr;
        t = tbl;
        this.sourceType = sourceType;

        var fileName = getFullFileNameForTable();
        try {
            fos = new FileOutputStream(fileName);
            fd = fos.getFD();
            var f = new FileWriter(fd);
            var bufferedWriter = new BufferedWriter(f);
            CSVFormat fmt = CSVFormat.POSTGRESQL_CSV
                    .builder()
                    .setNullString(ybVoyagerNullString)
                    .build();
            csvPrinter = new CSVPrinter(bufferedWriter, fmt);
            ArrayList<String> cols = t.getColumns();
            String header = String.join(fmt.getDelimiterString(), cols) + fmt.getRecordSeparator();
            LOGGER.debug("header = {}", header);
            f.write(header);
            es = ExportStatus.getInstance(dataDir);
            es.updateTableSnapshotWriterCreated(tbl, getFilenameForTable());

        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public void writeRecord(Record r) {
        // TODO: assert r.table = table
        try {
            csvPrinter.printRecord(r.getValueFieldValues());
            es.updateTableSnapshotRecordWritten(t);
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    private String getFilenameForTable() {
        String fileName =  t.tableName + "_data.sql";
        if ((sourceType.equals("postgresql")) && (!t.schemaName.equals("public"))){
            fileName = t.schemaName + "." + fileName;
        }
        return fileName;
    }

    private String getFullFileNameForTable() {
        return dataDir + "/" + getFilenameForTable();
    }

    @Override
    public void flush() {
        try {
            csvPrinter.flush();
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public void close() {
        try {
            String eof = "\\.";
            csvPrinter.getOut().append(eof);
            csvPrinter.println();
            csvPrinter.println();

            flush();
            sync();
            csvPrinter.close(true);
            fos.close();
            LOGGER.info("Closing snapshot file for table {}", t);
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public void sync() {
        try {
            fd.sync();
        }
        catch (SyncFailedException e) {
            throw new RuntimeException(e);
        }
    }
}
