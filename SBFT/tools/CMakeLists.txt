project(concord_bft_tools LANGUAGES CXX)


# GRPDC 
if(NOT MSVC)
  set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -std=c++11 -pthread")
else()
  add_definitions(-D_WIN32_WINNT=0x600)
endif()

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

# compile GRPC stuff

get_filename_component(rc_proto "../bftengine/tests/simpleKVBC/rpc/polaris.proto" ABSOLUTE)
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
include_directories("${CMAKE_CURRENT_BINARY_DIR}")


add_executable(GenerateConcordKeys
               GenerateConcordKeys.cpp
               KeyfileIOUtils.hpp
               KeyfileIOUtils.cpp)
target_include_directories(GenerateConcordKeys
                           PRIVATE
                           ${threshsign_SOURCE_DIR}/src
                           ${threshsign_SOURCE_DIR}/lib
                           ${threshsign_SOURCE_DIR}/include
                           ${bftengine_SOURCE_DIR}/src
                           ${bftengine_SOURCE_DIR}/include)
target_link_libraries(GenerateConcordKeys
                      PUBLIC
                      threshsign)
link_with_relic_library(GenerateConcordKeys)
set_target_properties(GenerateConcordKeys
                      PROPERTIES
                      RUNTIME_OUTPUT_DIRECTORY
                      .)

add_executable(TestGeneratedKeys
               TestGeneratedKeys.cpp
               KeyfileIOUtils.hpp
               KeyfileIOUtils.cpp)
target_include_directories(TestGeneratedKeys
                           PRIVATE
                           ${threshsign_SOURCE_DIR}/src
                           ${threshsign_SOURCE_DIR}/lib
                           ${threshsign_SOURCE_DIR}/include
                           ${bftengine_SOURCE_DIR}/src
                           ${bftengine_SOURCE_DIR}/include)
target_link_libraries(TestGeneratedKeys
                      PUBLIC
                      threshsign
                      corebft)
link_with_relic_library(TestGeneratedKeys)
set_target_properties(TestGeneratedKeys
                      PROPERTIES
                      RUNTIME_OUTPUT_DIRECTORY
                      .)

file(COPY ${CMAKE_CURRENT_SOURCE_DIR}/testKeyGeneration.sh
       DESTINATION ${CMAKE_CURRENT_BINARY_DIR}/)


#//Verifier##################
add_executable(SigVerifier
               SigVerifier.cpp
               KeyfileIOUtils.hpp
               KeyfileIOUtils.cpp
               ${rc_proto_srcs}
                 ${rc_grpc_srcs}
)

target_include_directories(SigVerifier
                           PRIVATE
                           ${threshsign_SOURCE_DIR}/src
                           ${threshsign_SOURCE_DIR}/lib
                           ${threshsign_SOURCE_DIR}/include
                           ${bftengine_SOURCE_DIR}/src
                           ${bftengine_SOURCE_DIR}/include)
target_link_libraries(SigVerifier
                      PUBLIC
                      threshsign
                      corebft   ${_GRPC_GRPCPP_UNSECURE} ${PROTOBUF_LIBRARY})
link_with_relic_library(SigVerifier)
set_target_properties(SigVerifier
                      PROPERTIES
                      RUNTIME_OUTPUT_DIRECTORY
                      .)
