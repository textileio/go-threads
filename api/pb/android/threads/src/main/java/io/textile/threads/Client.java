package io.textile.threads;

import java.io.ByteArrayOutputStream;
import java.util.Arrays;
import java.util.LinkedList;
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
import io.textile.threads_grpc.APIGrpc;
import io.textile.threads_grpc.NewStoreReply;
import io.textile.threads_grpc.NewStoreRequest;


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


    public String NewStore () {
         NewStoreRequest.Builder request = NewStoreRequest.newBuilder();
         NewStoreReply reply = blockingStub.newStore(request.build());
         return reply.getID();
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

    public Boolean connected() {
        return state == ClientState.Connected;
    }
}