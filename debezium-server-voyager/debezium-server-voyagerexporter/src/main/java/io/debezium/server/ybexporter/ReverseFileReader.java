package io.debezium.server.ybexporter;

import java.io.RandomAccessFile;
import java.nio.MappedByteBuffer;
import java.nio.channels.FileChannel;
import java.io.IOException;

public class ReverseFileReader implements AutoCloseable {
    private MappedByteBuffer buffer;
    private long currentPosition;

    public ReverseFileReader(String filePath) throws IOException {
        try (RandomAccessFile raf = new RandomAccessFile(filePath, "r");
                FileChannel channel = raf.getChannel()) {
            long fileSize = channel.size();
            this.buffer = channel.map(FileChannel.MapMode.READ_ONLY, 0, fileSize);
            this.currentPosition = fileSize - 1;
        }
    }

    public Boolean hasNext() {
        return currentPosition >= 0;
    }

    public String nextLine() {
        StringBuilder sb = new StringBuilder();
        try {
            while (currentPosition >= 0) {
                char c = (char) buffer.get((int) currentPosition);

                if (currentPosition == 0) { // If file start
                    sb.insert(0, c);
                    String currentLine = sb.toString();
                    currentPosition--;
                    return currentLine;
                } else if (c == '\n') { // If new line
                    String currentLine = sb.toString();
                    currentPosition--;
                    return currentLine;
                } else {
                    sb.insert(0, c);
                }

                currentPosition--;
            }
        } catch (Exception e) {
            return null;
        }
        return null;
    }

    @Override
    public void close() {
        buffer.clear();
    }
}
