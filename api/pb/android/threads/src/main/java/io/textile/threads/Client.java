package io.textile.threads;

import java.util.Arrays;
import java.util.function.Consumer;
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
    private static Config config;
    enum ClientState {
        Connected, Idle
    }
    public static ClientState state = ClientState.Idle;

    /**
     * Initialize a new Client
     */
    public Client() {
        this.config = new DefaultConfig();
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
     * @param ready
     */
    public void init(Consumer<Boolean> ready) {
        config.init((success) -> {
            if (success == true) {
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
                ready.accept(true);
            }
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

    public void StartFromAddress (String storeID, String address, ByteString followKey, ByteString readKey, StreamObserver<StartFromAddressReply> responseObserver) {
        StartFromAddressRequest.Builder request = StartFromAddressRequest.newBuilder();
        request.setStoreID(storeID);
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

    public void GetStoreLink (String dbID, StreamObserver<GetStoreLinkReply> responseObserver) {
        GetStoreLinkRequest.Builder request = GetStoreLinkRequest.newBuilder();
        request.setDBID(dbID);
        asyncStub.getStoreLink(request.build(), responseObserver);
    }

    public ModelCreateReply ModelCreateSync (String dbID, String modelName, String[] values) {
        ModelCreateRequest.Builder request = ModelCreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        ModelCreateReply reply = blockingStub.modelCreate(request.build());
        return reply;
    }

    public void ModelCreate (String dbID, String modelName, String[] values, StreamObserver<ModelCreateReply> responseObserver) {
        ModelCreateRequest.Builder request = ModelCreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        asyncStub.modelCreate(request.build(), responseObserver);
    }

    public ModelSaveReply ModelSaveSync (String dbID, String modelName, String[] values) {
        ModelSaveRequest.Builder request = ModelSaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        ModelSaveReply reply = blockingStub.modelSave(request.build());
        return reply;
    }

    public void ModelSave (String dbID, String modelName, String[] values, StreamObserver<ModelSaveReply> responseObserver) {
        ModelSaveRequest.Builder request = ModelSaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        ModelSaveReply reply = blockingStub.modelSave(request.build());
        asyncStub.modelSave(request.build(), responseObserver);
    }

    public boolean ModelHasSync (String dbID, String modelName, String[] entityIDs) {
        ModelHasRequest.Builder request = ModelHasRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        for (int i = 1; i < entityIDs.length; i++) {
            request.setEntityIDs(i, entityIDs[i]);
        }
        ModelHasReply reply = blockingStub.modelHas(request.build());
        return reply.getExists();
    }

    public void ModelHas (String dbID, String modelName, String[] entityIDs, StreamObserver<ModelHasReply> responseObserver) {
        ModelHasRequest.Builder request = ModelHasRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        for (int i = 1; i < entityIDs.length; i++) {
            request.setEntityIDs(i, entityIDs[i]);
        }
        asyncStub.modelHas(request.build(), responseObserver);
    }

    public ModelFindByIDReply ModelFindByIDSync (String dbID, String modelName, String entityID) {
        ModelFindByIDRequest.Builder request = ModelFindByIDRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.setEntityID(entityID);
        ModelFindByIDReply reply = blockingStub.modelFindByID(request.build());
        return reply;
    }
  
    public void ModelFindByID (String dbID, String modelName, String entityID, StreamObserver<ModelFindByIDReply> responseObserver) {
        ModelFindByIDRequest.Builder request = ModelFindByIDRequest.newBuilder();
        request.setDBID(storeID);
        request.setModelName(modelName);
        request.setEntityID(entityID);
        asyncStub.modelFindByID(request.build(), responseObserver);
    }

    public ModelFindReply ModelFindSync (String dbID, String modelName, ByteString query) {
        ModelFindRequest.Builder request = ModelFindRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.setQueryJSON(query);
        ModelFindReply reply = blockingStub.modelFind(request.build());
        return reply;
    }

    public void ModelFind (String dbID, String modelName, ByteString query, StreamObserver<ModelFindReply> responseObserver) {
        ModelFindRequest.Builder request = ModelFindRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.setQueryJSON(query);
        asyncStub.modelFind(request.build(), responseObserver);
    }

    public void RegisterSchemaSync (String dbID, String name, String schema) {
        RegisterSchemaRequest.Builder request = RegisterSchemaRequest.newBuilder();
        request.setDBID(dbID);
        request.setName(name);
        request.setSchema(schema);
        blockingStub.registerSchema(request.build());
        return;
    }

    public void RegisterSchema (String storeID, String name, String schema, StreamObserver<RegisterSchemaReply> responseObserver) {
        RegisterSchemaRequest.Builder request = RegisterSchemaRequest.newBuilder();
        request.setStoreID(storeID);
        request.setName(name);
        request.setSchema(schema);
        asyncStub.registerSchema(request.build(), responseObserver);
    }

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}
