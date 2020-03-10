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

    public CreateReply CreateSync (String dbID, String collectionName, String[] values) {
        CreateRequest.Builder request = CreateRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        CreateReply reply = blockingStub.create(request.build());
        return reply;
    }

    public SaveReply SaveSync (String dbID, String collectionName, String[] values) {
        SaveRequest.Builder request = SaveRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.addAllValues(Arrays.asList(values));
        SaveReply reply = blockingStub.save(request.build());
        return reply;
    }

    public boolean HasSync (String dbID, String collectionName, String[] entityIDs) {
        HasRequest.Builder request = HasRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        for (int i = 1; i < entityIDs.length; i++) {
            request.setEntityIDs(i, entityIDs[i]);
        }
        HasReply reply = blockingStub.has(request.build());
        return reply.getExists();
    }

    public FindByIDReply FindByIDSync (String dbID, String collectionName, String entityID) {
        FindByIDRequest.Builder request = FindByIDRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setEntityID(entityID);
        FindByIDReply reply = blockingStub.findByID(request.build());
        return reply;
    }

    public FindReply FindSync (String dbID, String collectionName, ByteString query) {
        FindRequest.Builder request = FindRequest.newBuilder();
        request.setDBID(dbID);
        request.setCollectionName(collectionName);
        request.setQueryJSON(query);
        FindReply reply = blockingStub.find(request.build());
        return reply;
    }

    public void NewCollectionSync (String dbID, String name, String schema) {
        NewCollectionRequest.Builder request = NewCollectionRequest.newBuilder();
        request.setDBID(dbID);
        request.setName(name);
        request.setSchema(schema);
        blockingStub.newCollection(request.build());
        return;
    }

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}