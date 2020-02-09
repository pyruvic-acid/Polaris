cmake_minimum_required (VERSION 3.2)
project(libsimpleKVBC VERSION 0.1.0.0 LANGUAGES CXX)



# GRPDC 
if(NOT MSVC)
  set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -std=c++11 -pthread")
else()
  add_definitions(-D_WIN32_WINNT=0x600)
endif()

## GRPC 
  include_directories(/home/name-removed/concord-installs/grpc)
  message(STATUS "Using gRPC via add_subdirectory.")
  
  # After using add_subdirectory, we can now use the grpc targets directly from
  # this build.
#  set(_PROTOBUF_LIBPROTOBUF libprotobuf)
    
    include(FindProtobuf)
    find_package(Protobuf REQUIRED)
    include_directories(${PROTOBUF_INCLUDE_DIR})
    set(_PROTOBUF_PROTOC  $<TARGET_FILE:protoc>)

    set(_GRPC_GRPCPP_UNSECURE grpc++_unsecure)
  set(_GRPC_CPP_PLUGIN_EXECUTABLE /usr/local/bin/grpc_cpp_plugin)
#    set(_GRPC_CPP_PLUGIN_EXECUTABLE grpc_cpp_plugin)



## ADD other stuff 
#if(USE_LOG4CPP)
#	target_compile_definitions(skvbc_client PUBLIC USE_LOG4CPP=1)
#endif()

#if(${USE_COMM_PLAIN_TCP})
#	target_compile_definitions(skvbc_client PUBLIC USE_COMM_PLAIN_TCP)
#endif()

#if(${USE_COMM_TLS_TCP})
#	target_compile_definitions(skvbc_client PUBLIC USE_COMM_TLS_TCP)
#endif()

# compile GRPC stuff

get_filename_component(rc_proto "../rpc/polaris.proto" ABSOLUTE)
get_filename_component(rc_proto_path "${rc_proto}" PATH)

# Generated sources
set(rc_proto_srcs "${CMAKE_CURRENT_BINARY_DIR}/polaris.pb.cc")
set(rc_proto_hdrs "${CMAKE_CURRENT_BINARY_DIR}/polaris.pb.h")
set(rc_grpc_srcs "${CMAKE_CURRENT_BINARY_DIR}/polaris.grpc.pb.cc")
set(rc_grpc_hdrs "${CMAKE_CURRENT_BINARY_DIR}/polaris.grpc.pb.h")

add_custom_command(
      OUTPUT "${rc_proto_srcs}" "${rc_proto_hdrs}" "${rc_grpc_srcs}" "${rc_grpc_hdrs}"
      COMMAND protoc
      ARGS --grpc_out "${CMAKE_CURRENT_BINARY_DIR}"
        --cpp_out "${CMAKE_CURRENT_BINARY_DIR}"
        -I "${rc_proto_path}"
        --plugin=protoc-gen-grpc=${_GRPC_CPP_PLUGIN_EXECUTABLE}
        "${rc_proto}"
      DEPENDS "${rc_proto}")

# Include generated *.pb.h files
include_directories("${CMAKE_CURRENT_BINARY_DIR}")



set(simpleKVBC_sources
	BlockchainDBAdapter.cpp
	ClientImp.cpp
	Comparators.cpp
	ReplicaImp.cpp
	InMemoryDBClient.cpp
	Slice.cpp
        ${rc_proto_srcs}
        ${rc_grpc_srcs}
	../../../../tools/KeyfileIOUtils.cpp
)

add_library(simpleKVBC ${simpleKVBC_sources})
target_link_libraries(simpleKVBC PUBLIC corebft threshsign util logging  ${_GRPC_GRPCPP_UNSECURE} ${PROTOBUF_LIBRARY})

target_include_directories(simpleKVBC PUBLIC .)
target_include_directories(simpleKVBC PUBLIC ../../../../tools)
target_include_directories(simpleKVBC PUBLIC ../include)
target_include_directories(simpleKVBC PUBLIC rpc)
