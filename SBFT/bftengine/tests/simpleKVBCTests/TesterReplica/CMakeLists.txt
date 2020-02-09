cmake_minimum_required (VERSION 3.2)
project(skvbc_replica VERSION 0.1.0.0 LANGUAGES CXX)


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
#  set(_GRPC_CPP_PLUGIN_EXECUTABLE /usr/local/bin/grpc_cpp_plugin)
    set(_GRPC_CPP_PLUGIN_EXECUTABLE $<TARGET_FILE:grpc_cpp_plugin>)



## ADD other stuff 


# compile GRPC stuff

get_filename_component(rc_proto "../../simpleKVBC/rpc/polaris.proto" ABSOLUTE)
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
        --plugin=protoc-gen-grpc=/usr/local/bin/grpc_cpp_plugin
        "${rc_proto}"
      DEPENDS "${rc_proto}")

# Include generated *.pb.h files
#include_directories("${CMAKE_CURRENT_BINARY_DIR}")
include_directories("${CMAKE_CURRENT_BINARY_DIR}")



set(replica_sources
	main.cpp
	../simpleKVBCTests.cpp
        ${rc_proto_srcs}
        ${rc_grpc_srcs}
	${CONFIG_FOLDER_PATH_VALUE}/test_comm_config.cpp
	${CONFIG_FOLDER_PATH_VALUE}/config_file_parser.cpp)

add_executable(skvbc_replica ${replica_sources})

if(USE_LOG4CPP)
	target_compile_definitions(skvbc_replica PUBLIC USE_LOG4CPP=1)
endif()

if(${USE_COMM_PLAIN_TCP})
	target_compile_definitions(skvbc_replica PUBLIC USE_COMM_PLAIN_TCP)
endif()

if(${USE_COMM_TLS_TCP})
	target_compile_definitions(skvbc_replica PUBLIC USE_COMM_TLS_TCP)
endif()

target_link_libraries(skvbc_replica LINK_PUBLIC simpleKVBC)

target_link_libraries(skvbc_replica PUBLIC corebft util)

target_include_directories(skvbc_replica PUBLIC ../../SimpleKVBC/include)
target_include_directories(skvbc_replica PUBLIC ..)
target_include_directories(skvbc_replica PUBLIC ${CONFIG_FOLDER_PATH_VALUE})
target_include_directories(skvbc_replica  PUBLIC rpc)


