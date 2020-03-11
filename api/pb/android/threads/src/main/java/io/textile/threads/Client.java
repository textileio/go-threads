package io.textile.threads;

import java.util.Arrays;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import android.arch.lifecycle.LifecycleObserver;
import com.google.protobuf.ByteString;
import io.grpc.CallCredentials;
import io.grpc.ManagedChannel;
import io.grpc.stub.StreamObserver;
import io.textile.threads_grpc.*;

/**
 * Provides top level access to the Textile API
 */
public class Client implements LifecycleObserver {
    private static APIGrpc.APIBlockingStub blockingStub;
    private static APIGrpc.APIStub asyncStub;
    private static Config config = new DefaultConfig();

    private ExecutorService executor
            = Executors.newSingleThreadExecutor();

    enum ClientState {
        Connected, Idle
    }
    public static ClientState state = ClientState.Idle;

    /**
     * Initialize a new Client
     */
    public Client() {
    }

    /**
     * Initialize a new Client
     * @param config is either a DefaultConfig for running threadsd or TextileConfig for using hosted.
     */
    public Client(Config config) {
        this.config = config;
    }

    /**
     *
     * @return the current session id or null
     */
    public String getSession() {
        return this.config.getSession();
    }
    /**
     * Method must be called before using the Client and while the device has an internet connection.
     */
    public Future<Void> init() throws Exception {
        return executor.submit(() -> {
            config.init();
            String session = config.getSession();
            ManagedChannel channel = config.getChannel();
            if (session != null) {
                CallCredentials bearer = new BearerToken(session);
                blockingStub = APIGrpc.newBlockingStub(channel)
                        .withCallCredentials(bearer);
                asyncStub = APIGrpc.newStub(channel)
                        .withCallCredentials(bearer);
            } else {
                blockingStub = APIGrpc.newBlockingStub(channel);
                asyncStub = APIGrpc.newStub(channel);
            }
            state = ClientState.Connected;
            return null;
        });
    }

    public String NewDBSync () {
        NewDBRequest.Builder request = NewDBRequest.newBuilder();
        NewDBReply reply = blockingStub.newDB(request.build());
        return reply.getID();
    }


    public void NewDB (StreamObserver<NewDBReply> responseObserver) {
        NewDBRequest.Builder request = NewDBRequest.newBuilder();
        asyncStub.newDB(request.build(), responseObserver);
    }

    public void StartSync (String dbID) {
        StartRequest.Builder request = StartRequest.newBuilder();
        request.setDBID(dbID);
        blockingStub.start(request.build());
        return;
    }

    public void Start (String dbID, StreamObserver<StartReply> responseObserver) {
        StartRequest.Builder request = StartRequest.newBuilder();
        request.setDBID(dbID);
        asyncStub.start(request.build(), responseObserver);
    }

    public void StartFromAddressSync (String dbID, String address, ByteString followKey, ByteString readKey) {
        StartFromAddressRequest.Builder request = StartFromAddressRequest.newBuilder();
        request.setDBID(dbID);
        request.setAddress(address);
        request.setFollowKey(followKey);
        request.setReadKey(readKey);
        blockingStub.startFromAddress(request.build());
        return;
    }

    public void StartFromAddress (String dbID, String address, ByteString followKey, ByteString readKey, StreamObserver<StartFromAddressReply> responseObserver) {
        StartFromAddressRequest.Builder request = StartFromAddressRequest.newBuilder();
        request.setDBID(dbID);
        request.setAddress(address);
        request.setFollowKey(followKey);
        request.setReadKey(readKey);
        asyncStub.startFromAddress(request.build(), responseObserver);
    }


    public GetDBLinkReply GetDBLinkSync (String dbID) {
        GetDBLinkRequest.Builder request = GetDBLinkRequest.newBuilder();
        request.setDBID(dbID);
        return blockingStub.getDBLink(request.build());
    }

    public void GetDBLink (String dbID, StreamObserver<GetDBLinkReply> responseObserver) {
        GetDBLinkRequest.Builder request = GetDBLinkRequest.newBuilder();
        request.setDBID(dbID);
        asyncStub.getDBLink(request.build(), responseObserver);
    }

    public CreateReply CreateSync (String dbID, String collectionName, String[] values) {
        CreateRequest.Builder request = CreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        CreateReply reply = blockingStub.create(request.build());
        return reply;
    }

    public void Create (String dbID, String collectionName, String[] values, StreamObserver<CreateReply> responseObserver) {
        CreateRequest.Builder request = CreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        asyncStub.create(request.build(), responseObserver);
    }

    public SaveReply SaveSync (String dbID, String collectionName, String[] values) {
        SaveRequest.Builder request = SaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        SaveReply reply = blockingStub.save(request.build());
        return reply;
    }

    public void Save (String dbID, String collectionName, String[] values, StreamObserver<SaveReply> responseObserver) {
        SaveRequest.Builder request = SaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        SaveReply reply = blockingStub.save(request.build());
        asyncStub.save(request.build(), responseObserver);
    }

    public boolean HasSync (String dbID, String collectionName, String[] instanceIDs) {
        HasRequest.Builder request = HasRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        for (int i = 1; i < instanceIDs.length; i++) {
            request.setInstanceIDs(i, instanceIDs[i]);
        }
        HasReply reply = blockingStub.has(request.build());
        return reply.getExists();
    }

    public void Has (String dbID, String collectionName, String[] instanceIDs, StreamObserver<HasReply> responseObserver) {
        HasRequest.Builder request = HasRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        for (int i = 1; i < instanceIDs.length; i++) {
            request.setInstanceIDs(i, instanceIDs[i]);
        }
        asyncStub.has(request.build(), responseObserver);
    }

    public FindByIDReply FindByIDSync (String dbID, String collectionName, String instanceID) {
        FindByIDRequest.Builder request = FindByIDRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setInstanceID(instanceID);
        FindByIDReply reply = blockingStub.findByID(request.build());
        return reply;
    }
  
    public void FindByID (String dbID, String collectionName, String instanceID, StreamObserver<FindByIDReply> responseObserver) {
        FindByIDRequest.Builder request = FindByIDRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setInstanceID(instanceID);
        asyncStub.findByID(request.build(), responseObserver);
    }

    public FindReply FindSync (String dbID, String collectionName, ByteString query) {
        FindRequest.Builder request = FindRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setQueryJSON(query);
        FindReply reply = blockingStub.find(request.build());
        return reply;
    }

    public void Find (String dbID, String collectionName, ByteString query, StreamObserver<FindReply> responseObserver) {
        FindRequest.Builder request = FindRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setQueryJSON(query);
        asyncStub.find(request.build(), responseObserver);
    }

    public void NewCollectionSync (String dbID, String name, String schema) {
        NewCollectionRequest.Builder request = NewCollectionRequest.newBuilder();
        request.setDBID(dbID);
        request.setName(name);
        request.setSchema(schema);
        blockingStub.newCollection(request.build());
        return;
    }

    public void NewCollection (String dbID, String name, String schema, StreamObserver<NewCollectionReply> responseObserver) {
        NewCollectionRequest.Builder request = NewCollectionRequest.newBuilder();
        request.setDBID(dbID);
        request.setName(name);
        request.setSchema(schema);
        asyncStub.newCollection(request.build(), responseObserver);
    }

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}
