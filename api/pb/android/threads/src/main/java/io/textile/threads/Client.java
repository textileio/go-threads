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

    public String NewStoreSync () {
        NewStoreRequest.Builder request = NewStoreRequest.newBuilder();
        NewStoreReply reply = blockingStub.newStore(request.build());
        return reply.getID();
    }

    public void StartSync (String storeID) {
        StartRequest.Builder request = StartRequest.newBuilder();
        request.setStoreID(storeID);
        blockingStub.start(request.build());
        return;
    }

    public void StartFromAddressSync (String storeID, String address, ByteString followKey, ByteString readKey) {
        StartFromAddressRequest.Builder request = StartFromAddressRequest.newBuilder();
        request.setStoreID(storeID);
        request.setAddress(address);
        request.setFollowKey(followKey);
        request.setReadKey(readKey);
        blockingStub.startFromAddress(request.build());
        return;
    }


    public GetStoreLinkReply GetStoreLinkSync (String storeID) {
        GetStoreLinkRequest.Builder request = GetStoreLinkRequest.newBuilder();
        request.setStoreID(storeID);
        return blockingStub.getStoreLink(request.build());
    }

    public ModelCreateReply ModelCreateSync (String storeID, String modelName, String[] values) {
        ModelCreateRequest.Builder request = ModelCreateRequest.newBuilder();
        request.setStoreID(storeID);
        request.setModelName(modelName);
        for (int i = 1; i < values.length; i++) {
            request.setValues(i, values[i]);
        }
        return blockingStub.modelCreate(request.build());
    }

    public ModelSaveReply ModelSaveSync (String storeID, String modelName, String[] values) {
        ModelSaveRequest.Builder request = ModelSaveRequest.newBuilder();
        request.setStoreID(storeID);
        request.setModelName(modelName);
        for (int i = 1; i < values.length; i++) {
            request.setValues(i, values[i]);
        }
        ModelSaveReply reply = blockingStub.modelSave(request.build());
        return reply;
    }

    public boolean ModelHasSync (String storeID, String modelName, String[] entityIDs) {
        ModelHasRequest.Builder request = ModelHasRequest.newBuilder();
        request.setStoreID(storeID);
        request.setModelName(modelName);
        for (int i = 1; i < entityIDs.length; i++) {
            request.setEntityIDs(i, entityIDs[i]);
        }
        ModelHasReply reply = blockingStub.modelHas(request.build());
        return reply.getExists();
    }

    public ModelFindByIDReply ModelFindByIDSync (String storeID, String modelName, String entityID) {
        ModelFindByIDRequest.Builder request = ModelFindByIDRequest.newBuilder();
        request.setStoreID(storeID);
        request.setModelName(modelName);
        request.setEntityID(entityID);
        ModelFindByIDReply reply = blockingStub.modelFindByID(request.build());
        return reply;
    }

    public ModelFindReply ModelFindSync (String storeID, String modelName, ByteString query) {
        ModelFindRequest.Builder request = ModelFindRequest.newBuilder();
        request.setStoreID(storeID);
        request.setModelName(modelName);
        request.setQueryJSON(query);
        ModelFindReply reply = blockingStub.modelFind(request.build());
        return reply;
    }

    public void RegisterSchemaSync (String storeID, String name, String schema) {
        RegisterSchemaRequest.Builder request = RegisterSchemaRequest.newBuilder();
        request.setStoreID(storeID);
        request.setName(name);
        request.setSchema(schema);
        blockingStub.registerSchema(request.build());
        return;
    }

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}