package io.textile.threads;

import java.io.ByteArrayOutputStream;
import java.util.Arrays;
import java.util.LinkedList;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;
import java.util.logging.Level;
import java.util.logging.Logger;

import android.arch.lifecycle.LifecycleObserver;

import com.google.protobuf.ByteString;

import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;

import java.util.Iterator;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

import io.grpc.stub.StreamObserver;
import io.textile.threads_grpc.*;


class ClientException extends Exception
{
    public ClientException(String message)
    {
        super(message);
    }
}

/**
 * Provides top level access to the Textile API
 */
public class Client implements LifecycleObserver {
    private static ManagedChannel channel;
    private static APIGrpc.APIBlockingStub blockingStub;
    private static APIGrpc.APIStub asyncStub;
    private static String grpcHost;
    private static int grpcPort;
    enum ClientState {
        Connected, Idle
    }
    public static ClientState state = ClientState.Idle;

    public Client(String host, int port) {
        grpcHost = host;
        grpcPort = port;
    }

    public void Connect() {
        channel = ManagedChannelBuilder
                .forAddress(grpcHost, Math.toIntExact(grpcPort))
                .usePlaintext()
                .build();
        blockingStub = APIGrpc.newBlockingStub(channel);
        asyncStub = APIGrpc.newStub(channel);
        state = ClientState.Connected;
    }

    public String NewDBSync () {
        NewDBRequest.Builder request = NewDBRequest.newBuilder();
        NewDBReply reply = blockingStub.newDB(request.build());
        return reply.getID();
    }

    public void StartSync (String dbID) {
        StartRequest.Builder request = StartRequest.newBuilder();
        request.setDBID(dbID);
        blockingStub.start(request.build());
        return;
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


    public GetDBLinkReply GetDBLinkSync (String dbID) {
        GetDBLinkRequest.Builder request = GetDBLinkRequest.newBuilder();
        request.setDBID(dbID);
        return blockingStub.getDBLink(request.build());
    }

    public ModelCreateReply ModelCreateSync (String dbID, String modelName, String[] values) {
        ModelCreateRequest.Builder request = ModelCreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        ModelCreateReply reply = blockingStub.modelCreate(request.build());
        return reply;
    }

    public ModelSaveReply ModelSaveSync (String dbID, String modelName, String[] values) {
        ModelSaveRequest.Builder request = ModelSaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.addAllValues(Arrays.asList(values));
        ModelSaveReply reply = blockingStub.modelSave(request.build());
        return reply;
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

    public ModelFindByIDReply ModelFindByIDSync (String dbID, String modelName, String entityID) {
        ModelFindByIDRequest.Builder request = ModelFindByIDRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.setEntityID(entityID);
        ModelFindByIDReply reply = blockingStub.modelFindByID(request.build());
        return reply;
    }

    public ModelFindReply ModelFindSync (String dbID, String modelName, ByteString query) {
        ModelFindRequest.Builder request = ModelFindRequest.newBuilder();
        request.setDBID(dbID);
        request.setModelName(modelName);
        request.setQueryJSON(query);
        ModelFindReply reply = blockingStub.modelFind(request.build());
        return reply;
    }

    public void RegisterSchemaSync (String dbID, String name, String schema) {
        RegisterSchemaRequest.Builder request = RegisterSchemaRequest.newBuilder();
        request.setDBID(dbID);
        request.setName(name);
        request.setSchema(schema);
        blockingStub.registerSchema(request.build());
        return;
    }

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}