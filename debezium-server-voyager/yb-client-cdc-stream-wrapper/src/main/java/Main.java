import com.google.common.net.HostAndPort;
import org.yb.client.AsyncYBClient;
import org.yb.client.ListTablesResponse;
import org.yb.client.YBClient;
import org.yb.client.YBTable;
import org.yb.master.MasterDdlOuterClass.ListTablesResponsePB.TableInfo;
import java.util.Set;   
import java.util.HashSet;

import java.util.Objects;


public class Main {
    public static void main(String[] args) throws Exception {
        CmdLineParams parameters = CmdLineParams.createFromArgs(args);
        AsyncYBClient asyncClient = new AsyncYBClient.AsyncYBClientBuilder(parameters.masterAddresses)
                .sslCertFile(parameters.certFilePath) // TODO: support sslClientCertFiles(clientCert, clientKey)
                .build();

        YBClient client = new YBClient(asyncClient);

        if (parameters.deleteStreamId != null) {
            Set<String> streamIds = new HashSet<String>();
            streamIds.add(parameters.deleteStreamId);
            client.deleteCDCStream(streamIds, false, true);
        }  else if (parameters.toCreate) {
            YBTable table = null;
            try {
                ListTablesResponse resp = client.getTablesList();
                for (TableInfo tableInfo : resp.getTableInfoList()) {
                    String qualifiedTableName = String.format("%s.%s", tableInfo.getPgschemaName(), tableInfo.getName());
                    if (Objects.equals(qualifiedTableName, parameters.tableName)) {
                        table =  client.openTableByUUID(tableInfo.getId().toStringUtf8());
                    }
                }
            } catch (Exception ex) {
                throw ex;
            }
            if (table == null) {
                throw new RuntimeException("No table found with the specified name");
            }
            String streamID = client.createCDCStream(table, parameters.dbName, "PROTO", "EXPLICIT", null).getStreamId();
            System.out.println("CDC Stream ID: " + streamID);
        } else if (parameters.listMasters) {
            String tserverNode = parameters.masterAddresses.split(",")[0].split(":")[0] + ":" + parameters.tserverPort;
            String masterAddressesList = client.getMasterAddresses(HostAndPort.fromString(tserverNode)); // {<ip1>:7100},{<ip2>:7100},{<ip3>:7100}
            masterAddressesList = masterAddressesList.replace("{", ""); //removing {}
            masterAddressesList = masterAddressesList.replace("}", "");
            System.out.println("Master Addresses: " + masterAddressesList);
        } else {
            throw new RuntimeException("Either create or delete stream id should be specified");
        }
    }
}
