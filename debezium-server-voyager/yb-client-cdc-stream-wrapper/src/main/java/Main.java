import com.google.common.net.HostAndPort;
import org.yb.client.AsyncYBClient;
import org.yb.client.ListTablesResponse;
import org.yb.client.YBClient;
import org.yb.client.ListCDCStreamsResponse;
import org.yb.client.YBTable;
import org.yb.master.MasterDdlOuterClass.ListTablesResponsePB.TableInfo;
import org.yb.client.GetNamespaceInfoResponse;
import org.yb.CommonTypes.YQLDatabase;
import org.yb.master.MasterReplicationOuterClass;
import java.util.Set;   
import java.util.HashSet;
import java.util.stream.Collectors;

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
            /*
                EXPLICIT 'ALL' can enable before image for the events
                Refer: https://docs.yugabyte.com/preview/admin/yb-admin/#enabling-before-image
            */
            String streamID = client.createCDCStream(table, parameters.dbName, "PROTO", "EXPLICIT", null).getStreamId();
            System.out.println("CDC Stream ID: " + streamID);
        } else if (parameters.listMasters) {
            String tserverNode = parameters.masterAddresses.split(",")[0].split(":")[0] + ":" + parameters.tserverPort;
            String masterAddressesList = client.getMasterAddresses(HostAndPort.fromString(tserverNode)); // {<ip1>:7100},{<ip2>:7100},{<ip3>:7100}
            masterAddressesList = masterAddressesList.replace("{", ""); //removing {}
            masterAddressesList = masterAddressesList.replace("}", "");
            System.out.println("Master Addresses: " + masterAddressesList);
        } else if (parameters.getNumOfCDCStreams) {
            String namespaceID = null;
            if(parameters.dbName != "") {
                GetNamespaceInfoResponse namespaceInfoResponse =
                    client.getNamespaceInfo(parameters.dbName, YQLDatabase.YQL_DATABASE_PGSQL);
                if (namespaceInfoResponse.hasError()) {
                    throw new RuntimeException(
                        String.format(
                            "Error getting namespace details for namespace: %s. Error: %s",
                            parameters.dbName,
                            namespaceInfoResponse.errorMessage()));
                    }
                namespaceID = namespaceInfoResponse.getNamespaceId();
            }
            System.out.println("db name:" + parameters.dbName + "namespaceID:" + namespaceID);
            ListCDCStreamsResponse cdcStreamsResponse = client.listCDCStreams(null, namespaceID, MasterReplicationOuterClass.IdTypePB.NAMESPACE_ID);
            if (cdcStreamsResponse.hasError()) {
              throw new RuntimeException("error getting the num of cdc streams");
            }
            System.out.println("Streams: " + cdcStreamsResponse.getStreams().size());
        } else {
            throw new RuntimeException("unknown paramter");

        }
    }
}
